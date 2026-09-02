package controller

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// 对齐 Java WebMvcInitializer 的静态资源目录前缀判断。
var staticDirPrefixes = []string{
	"/assets/", "/css/", "/js/", "/img/", "/images/", "/fonts/", "/static/",
}

// staticDir 前端静态资源根目录。可通过 STATIC_DIR 环境变量指定，
// 默认与可执行文件所在目录相同。
var staticDir = defaultStaticDir()

func defaultStaticDir() string {
	if v := os.Getenv("STATIC_DIR"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

// SetStaticDir 由 main 解析命令行参数后调用，覆盖静态资源根目录。
// 优先级：命令行参数 > STATIC_DIR 环境变量 > 可执行文件所在目录。
func SetStaticDir(dir string) {
	if dir != "" {
		staticDir = dir
	}
}

// serveStatic 从文件系统服务前端静态资源，并对 SPA 路由回退到 index.html。
// 对应 Java 的 WebMvcInitializer（其 /* 实际应表达为 /** 的语义）。
func serveStatic(r *ghttp.Request) {
	path := r.URL.Path
	rel := strings.TrimPrefix(path, "/")
	if rel == "" {
		rel = "index.html"
	}

	if file, ok := resolveStaticPath(rel); ok {
		if data, err := os.ReadFile(file); err == nil {
			writeStatic(r, rel, data)
			return
		}
	}

	// 未命中：静态资源请求直接 404；否则按 SPA 回退到首页。
	if strings.HasPrefix(path, "/v1/") || isStaticResourceRequest(path) {
		r.Response.WriteHeader(http.StatusNotFound)
		return
	}
	if file, ok := resolveStaticPath("index.html"); ok {
		if data, err := os.ReadFile(file); err == nil {
			writeStatic(r, "index.html", data)
			return
		}
	}
	r.Response.WriteHeader(http.StatusNotFound)
}

// resolveStaticPath 将 URL 相对路径解析为静态目录内的绝对路径，
// 防止路径穿越（../）。
func resolveStaticPath(rel string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		clean = "index.html"
	}
	if filepath.IsAbs(clean) {
		return "", false
	}
	full := filepath.Join(staticDir, clean)
	r, err := filepath.Rel(staticDir, full)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func writeStatic(r *ghttp.Request, rel string, data []byte) {
	contentType := mime.TypeByExtension(filepath.Ext(rel))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	r.Response.Header().Set("Content-Type", contentType)
	r.Response.Header().Set("Cache-Control", "max-age=0, must-revalidate")
	r.Response.WriteHeader(http.StatusOK)
	r.Response.Write(data)
}

func isStaticResourceRequest(path string) bool {
	for _, prefix := range staticDirPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
