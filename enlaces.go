package main

import (
	"net"
	"net/http"
	"regexp"
	"strings"
)

// Enlace lleva a un servicio que un stack publica a través de Traefik.
type Enlace struct {
	Texto string
	URL   string
}

var (
	// Regla de un router de Traefik, en etiquetas como lista (=) o como mapa (:).
	reglaTraefik = regexp.MustCompile(`(?m)traefik\.http\.routers\.[A-Za-z0-9_-]+\.rule\s*[=:]\s*(.+)$`)
	// Host(`nombre.completo`)
	hostCompleto = regexp.MustCompile("Host\\(`([A-Za-z0-9.-]+)`\\)")
	// HostRegexp(`^nombre\.`): solo el principio del nombre, el dominio es el del panel.
	// Categoría de un stack sin aplicación en el menú, en una etiqueta como
	// lista (=) o como mapa (:).
	etiquetaCategoria = regexp.MustCompile(`(?m)domo-board\.categoria\s*[=:]\s*["']?([^"'\n]*?)["']?\s*$`)
	hostPrefijo       = regexp.MustCompile("HostRegexp\\(`\\^([A-Za-z0-9-]+)\\\\\\.`\\)")
)

// categoriaStack devuelve la categoría que declara un compose con la etiqueta
// domo-board.categoria, o vacío si no la declara.
func categoriaStack(compose string) string {
	if m := etiquetaCategoria.FindStringSubmatch(compose); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// nombresTraefik devuelve los nombres que publican las reglas de Traefik de un
// compose. Los que acaban en punto son prefijos que se completan con el
// dominio del panel. Las reglas que no son de estas dos formas se ignoran.
func nombresTraefik(compose string) []string {
	var nombres []string
	for _, regla := range reglaTraefik.FindAllStringSubmatch(compose, -1) {
		for _, m := range hostCompleto.FindAllStringSubmatch(regla[1], -1) {
			nombres = append(nombres, m[1])
		}
		for _, m := range hostPrefijo.FindAllStringSubmatch(regla[1], -1) {
			nombres = append(nombres, m[1]+".")
		}
	}
	return nombres
}

// enlaces convierte los nombres en enlaces. Los prefijos usan el dominio con el
// que se ha abierto el panel; si se abre por IP o por un nombre sin dominio,
// como en un túnel SSH, no tienen enlace.
func enlaces(nombres []string, r *http.Request) []Enlace {
	esquema := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		esquema = "https"
	}
	dominio := dominioDe(r.Host)
	var resultado []Enlace
	for _, n := range nombres {
		host := n
		if strings.HasSuffix(n, ".") {
			if dominio == "" {
				continue
			}
			host = n + dominio
		}
		resultado = append(resultado, Enlace{Texto: host, URL: esquema + "://" + host + "/"})
	}
	return resultado
}

// dominioDe quita el primer nombre del host del panel: de panel.casa.example
// devuelve casa.example.
func dominioDe(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if net.ParseIP(host) != nil {
		return ""
	}
	_, dominio, ok := strings.Cut(host, ".")
	if !ok {
		return ""
	}
	return dominio
}
