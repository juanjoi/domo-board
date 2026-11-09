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
