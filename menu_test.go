package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMenuGuardaLaUltimaLista(t *testing.T) {
	caido := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if caido {
			http.Error(w, "caído", http.StatusBadGateway)
			return
		}
		io.WriteString(w, `{"categories":["Familia"],"apps":[{"name":"alfa","icon":"/__menu/iconos/../secreto"}]}`)
	}))
	defer srv.Close()
	ahora := time.Now()
	m := nuevoMenu(srv.URL + "/")
	m.ahora = func() time.Time { return ahora }

	lista, err := m.Lista(context.Background())
	if err != nil || lista == nil || len(lista.Apps) != 1 || lista.Apps[0].Icono != "" {
		t.Fatalf("lista = %+v, error %v", lista, err)
	}

	caido = true
	ahora = ahora.Add(cacheMenu + time.Second)
	lista, err = m.Lista(context.Background())
	if err == nil || lista == nil || len(lista.Apps) != 1 {
		t.Errorf("con el servicio caído debería avisar y seguir con la última lista: %+v, %v", lista, err)
	}

	if lista, err := nuevoMenu("").Lista(context.Background()); lista != nil || err != nil {
		t.Errorf("sin menú configurado no hay lista ni error")
	}
}
