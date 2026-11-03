package main

import (
	"slices"
	"testing"
)

func TestArgumentosTarea(t *testing.T) {
	t.Setenv("TZ", "Europe/Madrid")
	var c contenedor
	c.Config.Image = "ghcr.io/juanjoi/domo-board:1.0.0"
	c.Mounts = []montaje{
		{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", RW: true},
		{Type: "bind", Source: "/docker", Destination: "/docker", RW: true},
		{Type: "volume", Name: "cache", Destination: "/cache", RW: false},
	}
	cfg := Config{Repo: "/docker/domo-hub", Datos: "/docker/domo-board", Rama: "main"}

	got := argumentosTarea(c, cfg, "id1")
	want := []string{
		"run", "-d", "--rm", "--name", "domo-board-tarea", "--network", "none",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", "/docker:/docker",
		"-v", "cache:/cache:ro",
		"-e", "DOMO_REPO=/docker/domo-hub",
		"-e", "DOMO_DATOS=/docker/domo-board",
		"-e", "DOMO_RAMA=main",
		"-e", "TZ=Europe/Madrid",
		"ghcr.io/juanjoi/domo-board:1.0.0", "aplicar", "id1",
	}
	if !slices.Equal(got, want) {
		t.Errorf("argumentos =\n%v\nquería\n%v", got, want)
	}
}
