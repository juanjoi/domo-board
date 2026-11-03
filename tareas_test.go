package main

import (
	"testing"
	"time"
)

func TestRegistro(t *testing.T) {
	reg := Registro{Dir: t.TempDir()}
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	vieja := nuevaTarea("alfa", "c1", base)
	vieja.Estado = Correcta
	fallida := nuevaTarea("alfa", "c2", base.Add(time.Minute))
	fallida.Estado = Fallida
	beta := nuevaTarea("beta", "c3", base.Add(2*time.Minute))
	for _, tarea := range []Tarea{vieja, fallida, beta} {
		if err := reg.Guardar(tarea); err != nil {
			t.Fatal(err)
		}
	}

	todas, err := reg.Todas()
	if err != nil || len(todas) != 3 || todas[0].ID != beta.ID {
		t.Fatalf("Todas = %v, %v", todas, err)
	}
	desp := desplegados(todas)
	// La fallida no cuenta: alfa sigue en su última correcta. Beta está en curso.
	if desp["alfa"].Commit != "c1" || len(desp) != 1 {
		t.Errorf("desplegados = %v", desp)
	}
	if _, err := reg.Leer("../../etc/passwd"); err == nil {
		t.Error("Leer debería rechazar un id no válido")
	}
}

func TestRegistroVacio(t *testing.T) {
	todas, err := Registro{Dir: t.TempDir() + "/no-existe"}.Todas()
	if err != nil || todas != nil {
		t.Errorf("Todas = %v, %v", todas, err)
	}
}
