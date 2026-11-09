# domo-board

Aplicación web para gestionar un hub domótico basado en Docker Compose. La primera versión actualiza los stacks con un botón; más adelante podrá activar y desactivar aplicaciones. Repositorio público: no debe contener datos de ninguna casa concreta (nombres de servicios, dominios, IPs ni secretos).

## Convenciones

- Todo en español: documentación, mensajes de commit y textos de la interfaz.
- Go con la librería estándar; cualquier dependencia externa necesita un motivo claro.
- Versiones con etiquetas `vX.Y.Z`. GitHub Actions publica la imagen en GitHub Container Registry al etiquetar.

## Estructura

- `main.go`: configuración y órdenes (`servir`, `aplicar <tarea>`).
- `git.go`: el clon del repositorio de despliegue (buscar, stacks, cambios, restaurar).
- `tareas.go`: las tareas, un JSON y un log por tarea en la carpeta de datos.
- `docker.go` y `aplicar.go`: lanzar la tarea en un contenedor efímero y lo que hace dentro.
- `web.go` y `plantillas/`: la interfaz, HTML generado en el servidor sin JavaScript.
- `enlaces.go`: los enlaces a los servicios publicados, sacados de las reglas de Traefik.
- Pruebas junto a cada fichero (`*_test.go`); usan `git` real en carpetas temporales.
