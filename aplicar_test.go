package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"
)

func TestVaciarCola(t *testing.T) {
	reg := Registro{Dir: t.TempDir()}
	base := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	muerta := nuevaTarea("vieja", "c0", base.Add(-time.Hour))
	muerta.Estado = EnCurso
	lote := nuevoLote([]string{"a", "b", "c"}, "c1", base)
	suelta := nuevaTarea("d", "c1", base.Add(time.Second))
	for _, ta := range append(lote, muerta, suelta) {
		if err := reg.Guardar(ta); err != nil {
			t.Fatal(err)
		}
	}

	var aplicadas []string
	err := vaciarCola(context.Background(), reg, func(ta Tarea, salida io.Writer) error {
		aplicadas = append(aplicadas, ta.Stack)
		fmt.Fprintln(salida, "aplicando", ta.Stack)
		if ta.Stack == "b" {
			return errors.New("falla b")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// c se cancela por ir detrás de b en su lote; d es de otro lote y sigue.
	if want := []string{"a", "b", "d"}; !slices.Equal(aplicadas, want) {
		t.Errorf("aplicadas = %v, quería %v", aplicadas, want)
	}
	estados := map[string]string{}
	tareas, _ := reg.Todas()
	for _, ta := range tareas {
		estados[ta.Stack] = ta.Estado
	}
	want := map[string]string{"vieja": Fallida, "a": Correcta, "b": Fallida, "c": Cancelada, "d": Correcta}
	for stack, estado := range want {
		if estados[stack] != estado {
			t.Errorf("%s: estado %q, quería %q", stack, estados[stack], estado)
		}
	}
	if log := leerFinal(reg.RutaLog(lote[1].ID), 1000); log != "aplicando b\n\nError: falla b\n" {
		t.Errorf("log de b = %q", log)
	}
}
