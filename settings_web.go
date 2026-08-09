package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const defaultSettingsHTTPPort = 6655

const (
	settingsHTTPShutdownTimeout = 2 * time.Second
	settingsMaxRequestBytes     = 2 << 20
)

//go:embed web/settings/*
var settingsWebAssets embed.FS

var settingsWeb = newSettingsWebService(currentKeywordStore, openDefaultBrowser)

type settingsWebStatus struct {
	Running bool
	Port    int
	URL     string
}

type settingsWebService struct {
	mu            sync.Mutex
	storeProvider func() *RuleStore
	opener        func(string) error
	server        *http.Server
	port          int
	token         string
	url           string
}

func newSettingsWebService(storeProvider func() *RuleStore, opener func(string) error) *settingsWebService {
	return &settingsWebService{
		storeProvider: storeProvider,
		opener:        opener,
	}
}

func parseSettingsPort(text string) (int, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return defaultSettingsHTTPPort, nil
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("端口必须是数字")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("端口必须在 1 到 65535 之间")
	}
	return port, nil
}

func checkSettingsPortAvailable(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("端口必须在 1 到 65535 之间")
	}
	listener, err := net.Listen("tcp", settingsListenAddress(port))
	if err != nil {
		return fmt.Errorf("端口 %d 不可用：%w", port, err)
	}
	return listener.Close()
}

func (service *settingsWebService) Start(port int) (string, error) {
	if service == nil {
		return "", errors.New("设置服务未初始化")
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("端口必须在 1 到 65535 之间")
	}
	store := service.currentStore()
	if store == nil {
		return "", errors.New("关键词规则尚未初始化，请重新启用插件后再打开设置")
	}

	token, err := newSettingsToken()
	if err != nil {
		return "", err
	}
	listener, err := net.Listen("tcp", settingsListenAddress(port))
	if err != nil {
		return "", fmt.Errorf("启动 HTTP 服务失败：%w", err)
	}

	service.mu.Lock()
	if service.server != nil {
		service.mu.Unlock()
		_ = listener.Close()
		return "", errors.New("HTTP 服务已在运行")
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	rawURL := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", actualPort, token)
	server := &http.Server{
		Handler:           service.routes(store, token),
		ReadHeaderTimeout: 5 * time.Second,
	}
	service.server = server
	service.port = actualPort
	service.token = token
	service.url = rawURL
	opener := service.opener
	service.mu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			service.clearIfCurrent(server)
		}
	}()

	if opener != nil {
		if err := opener(rawURL); err != nil {
			return rawURL, fmt.Errorf("HTTP 服务已启动，但打开浏览器失败：%w", err)
		}
	}
	return rawURL, nil
}

func (service *settingsWebService) Stop() error {
	if service == nil {
		return nil
	}
	service.mu.Lock()
	server := service.server
	service.server = nil
	service.port = 0
	service.token = ""
	service.url = ""
	service.mu.Unlock()

	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsHTTPShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (service *settingsWebService) Status() settingsWebStatus {
	if service == nil {
		return settingsWebStatus{}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return settingsWebStatus{
		Running: service.server != nil,
		Port:    service.port,
		URL:     service.url,
	}
}

func (service *settingsWebService) currentStore() *RuleStore {
	if service.storeProvider == nil {
		return nil
	}
	return service.storeProvider()
}

func (service *settingsWebService) clearIfCurrent(server *http.Server) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.server == server {
		service.server = nil
		service.port = 0
		service.token = ""
		service.url = ""
	}
}

func (service *settingsWebService) routes(store *RuleStore, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveSettingsIndex)
	mux.HandleFunc("/styles.css", serveSettingsAsset("web/settings/styles.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/app.js", serveSettingsAsset("web/settings/app.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("/api/rules", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("token") != token {
			writeSettingsJSONError(writer, http.StatusForbidden, "无效的访问令牌")
			return
		}
		switch request.Method {
		case http.MethodGet:
			writeSettingsJSON(writer, http.StatusOK, store.Snapshot())
		case http.MethodPut:
			var rules []KeywordRule
			reader := http.MaxBytesReader(writer, request.Body, settingsMaxRequestBytes)
			if err := json.NewDecoder(reader).Decode(&rules); err != nil {
				writeSettingsJSONError(writer, http.StatusBadRequest, "规则 JSON 无效")
				return
			}
			if err := store.Replace(rules); err != nil {
				writeSettingsJSONError(writer, http.StatusBadRequest, err.Error())
				return
			}
			writeSettingsJSON(writer, http.StatusOK, store.Snapshot())
		default:
			writer.Header().Set("Allow", "GET, PUT")
			writeSettingsJSONError(writer, http.StatusMethodNotAllowed, "不支持的请求方法")
		}
	})
	return mux
}

func serveSettingsIndex(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	serveSettingsAsset("web/settings/index.html", "text/html; charset=utf-8")(writer, request)
}

func serveSettingsAsset(path, contentType string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := settingsWebAssets.ReadFile(path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", contentType)
		_, _ = writer.Write(data)
	}
}

func writeSettingsJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeSettingsJSONError(writer http.ResponseWriter, status int, message string) {
	writeSettingsJSON(writer, status, map[string]string{"error": message})
}

func settingsListenAddress(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func newSettingsToken() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("生成访问令牌失败：%w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func startSettingsWebService(port int) (string, error) {
	return settingsWeb.Start(port)
}

func stopSettingsWebService() error {
	return settingsWeb.Stop()
}

func settingsWebServiceStatus() settingsWebStatus {
	return settingsWeb.Status()
}

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW    = shell32.NewProc("ShellExecuteW")
	settingsOpenVerb     = utf16Ptr("open")
	settingsBrowserParam = uintptr(0)
)

func openDefaultBrowser(rawURL string) error {
	target, err := syscall.UTF16PtrFromString(rawURL)
	if err != nil {
		return err
	}
	result, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(settingsOpenVerb)),
		uintptr(unsafe.Pointer(target)),
		settingsBrowserParam,
		settingsBrowserParam,
		swShow,
	)
	if result <= 32 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("ShellExecuteW 失败，返回码 %d", result)
	}
	return nil
}
