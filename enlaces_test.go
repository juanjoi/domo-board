package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestNombresTraefik(t *testing.T) {
	compose := "services:\n" +
		"  web:\n" +
		"    labels:\n" +
		"      - traefik.enable=true\n" +
		"      - traefik.http.routers.web.rule=HostRegexp(`^web\\.`)\n" +
		"      - traefik.http.services.web.loadbalancer.server.port=80\n" +
		"  api:\n" +
		"    labels:\n" +
		"      traefik.http.routers.api.rule: Host(`api.example.com`) || Host(`otra.example.com`)\n" +
		"  raro:\n" +
		"    labels:\n" +
		"      - traefik.http.routers.raro.rule=HostRegexp(`^(a|b)\\.`)\n" +
		"      - traefik.http.routers.ruta.rule=PathPrefix(`/x`)\n"
	quiero := []string{"web.", "api.example.com", "otra.example.com"}
	if got := nombresTraefik(compose); !slices.Equal(got, quiero) {
		t.Errorf("nombresTraefik = %v, quería %v", got, quiero)
	}
	if got := nombresTraefik("services:\n  x:\n    image: x\n"); got != nil {
		t.Errorf("sin etiquetas = %v", got)
	}
}

func TestEnlaces(t *testing.T) {
	nombres := []string{"web.", "api.example.com"}
	casos := []struct {
		host, proto string
		quiero      []string
	}{
		{"panel.casa.example", "", []string{"http://web.casa.example/", "http://api.example.com/"}},
		{"panel.casa.example:8080", "https", []string{"https://web.casa.example/", "https://api.example.com/"}},
		{"127.0.0.1:8099", "", []string{"http://api.example.com/"}},
		{"localhost:8099", "", []string{"http://api.example.com/"}},
	}
	for _, c := range casos {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = c.host
		if c.proto != "" {
			r.Header.Set("X-Forwarded-Proto", c.proto)
		}
		var urls []string
		for _, e := range enlaces(nombres, r) {
			urls = append(urls, e.URL)
		}
		if !slices.Equal(urls, c.quiero) {
			t.Errorf("%s: %v, quería %v", c.host, urls, c.quiero)
		}
	}
}

func TestCategoriaStack(t *testing.T) {
	casos := map[string]string{
		"    labels:\n      - domo-board.categoria=Infraestructura\n": "Infraestructura",
		"    labels:\n      domo-board.categoria: \"Media\"\n":        "Media",
		"    labels:\n      - \"domo-board.categoria=Casa\"\n":        "Casa",
		"    labels:\n      - traefik.enable=true\n":                  "",
	}
	for compose, quiere := range casos {
		if got := categoriaStack(compose); got != quiere {
			t.Errorf("categoriaStack(%q) = %q, quiere %q", compose, got, quiere)
		}
	}
}

func TestDominioDe(t *testing.T) {
	casos := map[string]string{
		"panel.casa.example":      "casa.example",
		"Panel.Casa.Example:8080": "casa.example",
		"panel.internal":          "internal",
		"panel":                   "",
		"127.0.0.1:8099":          "",
		"[::1]:8099":              "",
		"panel.casa.example.":     "",
		"panel.casa_example":      "",
		"panel.-casa.example":     "",
		"panel.casa..example":     "",
		"panel.casa$(id).example": "",
	}
	for host, quiero := range casos {
		if d := dominioDe(host); d != quiero {
			t.Errorf("dominioDe(%q) = %q, quería %q", host, d, quiero)
		}
	}
}

func TestIconoStack(t *testing.T) {
	casos := map[string]string{
		"    labels:\n      - domo-board.icono=libretranslate.svg\n":      "/__menu/iconos/libretranslate.svg",
		"    labels:\n      domo-board.icono: \"postgres.png\"\n":         "/__menu/iconos/postgres.png",
		"    labels:\n      - \"domo-board.icono=mqtt.webp\"\n":           "/__menu/iconos/mqtt.webp",
		"    labels:\n      - domo-board.icono=../secreto.svg\n":          "",
		"    labels:\n      - domo-board.icono=..svg\n":                   "",
		"    labels:\n      - domo-board.icono=icono.exe\n":               "",
		"    labels:\n      - domo-board.icono=https://x.example/a.svg\n": "",
		"    labels:\n      - traefik.enable=true\n":                      "",
	}
	for compose, quiere := range casos {
		if got := iconoStack(compose); got != quiere {
			t.Errorf("iconoStack(%q) = %q, quiere %q", compose, got, quiere)
		}
	}
}
