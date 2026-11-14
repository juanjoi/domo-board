package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	cacheMenu      = 30 * time.Second
	esperaMenu     = 3 * time.Second
	maxMenu        = 1 << 20
	rutaIconos     = "/__menu/iconos/"
	categoriaOtras = "Otras"
)

// AppMenu es una aplicación de la lista del menú común: el principio de su
// nombre, o su nombre completo si se publica con Host, y lo que dice su
// middleware del menú.
type AppMenu struct {
	Nombre    string `json:"name"`
	Host      string `json:"host"`
	Titulo    string `json:"title"`
	Categoria string `json:"category"`
	Icono     string `json:"icon"`
}

// ListaMenu es lo que devuelve el servicio del menú en /__menu/apps.json.
type ListaMenu struct {
	Categorias []string  `json:"categories"`
	Apps       []AppMenu `json:"apps"`
}

// Menu pide la lista de aplicaciones al servicio del menú común y la guarda un
// rato. Si el servicio no responde, sigue usando la última que consiguió.
type Menu struct {
	URL     string // dirección base del servicio, vacía si no hay menú
	cliente *http.Client

	mu     sync.Mutex
	lista  *ListaMenu
	pedida time.Time
	ahora  func() time.Time
}

func nuevoMenu(url string) *Menu {
	return &Menu{URL: strings.TrimRight(url, "/"), cliente: &http.Client{Timeout: esperaMenu}, ahora: time.Now}
}

// Lista devuelve la lista del menú, o nil si no hay menú configurado o nunca
// se pudo leer. El error dice por qué no está al día.
func (m *Menu) Lista(ctx context.Context) (*ListaMenu, error) {
	if m == nil || m.URL == "" {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lista != nil && m.ahora().Sub(m.pedida) < cacheMenu {
		return m.lista, nil
	}
	lista, err := m.pedir(ctx)
	if err != nil {
		return m.lista, fmt.Errorf("No se pudo leer la lista de aplicaciones del menú: %w", err)
	}
	m.lista, m.pedida = lista, m.ahora()
	return lista, nil
}

func (m *Menu) pedir(ctx context.Context) (*ListaMenu, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL+"/__menu/apps.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.cliente.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respuesta %s", resp.Status)
	}
	var lista ListaMenu
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMenu)).Decode(&lista); err != nil {
		return nil, err
	}
	for i, a := range lista.Apps {
		// Solo iconos del propio servicio, que se sirven en el mismo origen.
		if !strings.HasPrefix(a.Icono, rutaIconos) || strings.Contains(a.Icono, "..") {
			lista.Apps[i].Icono = ""
		}
	}
	return &lista, nil
}
