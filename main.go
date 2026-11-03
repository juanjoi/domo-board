// domo-board es el panel del hub domótico: muestra los stacks del repositorio
// de despliegue y los actualiza con un botón.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "time/tzdata" // la imagen no trae zonas horarias y se usa TZ
)

// Config sale de variables de entorno.
type Config struct {
	Repo      string // clon del repositorio de despliegue
	Datos     string // carpeta de datos del panel
	Rama      string
	Direccion string
}

func leerConfig() (Config, error) {
	c := Config{
		Repo:      os.Getenv("DOMO_REPO"),
		Datos:     os.Getenv("DOMO_DATOS"),
		Rama:      valorOPorDefecto(os.Getenv("DOMO_RAMA"), "main"),
		Direccion: valorOPorDefecto(os.Getenv("DOMO_DIRECCION"), ":8080"),
	}
	if c.Repo == "" || c.Datos == "" {
		return c, fmt.Errorf("faltan DOMO_REPO o DOMO_DATOS")
	}
	return c, nil
}

func valorOPorDefecto(v, porDefecto string) string {
	if v == "" {
		return porDefecto
	}
	return v
}

func main() {
	log.SetFlags(0)
	cfg, err := leerConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, parar := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer parar()

	orden := "servir"
	if len(os.Args) > 1 {
		orden = os.Args[1]
	}
	switch {
	case orden == "servir" && len(os.Args) <= 2:
		err = servir(ctx, cfg)
	case orden == "aplicar" && len(os.Args) == 3:
		err = aplicar(ctx, cfg, os.Args[2])
	default:
		err = fmt.Errorf("uso: domo-board [servir | aplicar <tarea>]")
	}
	if err != nil {
		log.Fatal(err)
	}
}
