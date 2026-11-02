# Panel del hub domótico

Aplicación web pequeña para gestionar un hub domótico basado en Docker Compose. Se despliega como un stack más y lee los stacks de un repositorio de despliegue clonado en el servidor.

## Funciones

| Función | Estado |
| --- | --- |
| Ver los stacks y si tienen cambios sin desplegar | Primera versión |
| Actualizar un stack con un botón, incluido el propio panel | Primera versión |
| Activar y desactivar aplicaciones | Más adelante |

## Actualizar stacks

- **Descubre los stacks:** recorre `stacks/*/docker-compose.yml` del repositorio de despliegue, así que un stack nuevo aparece sin darlo de alta.
- **Muestra el estado:** compara el commit con el que se desplegó cada stack con el último de `main`, y avisa de los stacks que ya no están en el repositorio sin borrarlos.
- **Actualiza con un botón:** lanza `git pull` y `docker compose up -d` en un contenedor efímero, para que la tarea termine aunque el panel se reinicie al actualizarse a sí mismo.
- **Solo acepta nombres de stacks existentes:** tiene acceso al socket de Docker y, por tanto, control total del servidor.

## Detalles técnicos

- Go con la librería estándar, sin dependencias externas.
- La imagen lleva el binario, `git` y el cliente de Docker. GitHub Actions la publica en GitHub Container Registry al etiquetar una versión `vX.Y.Z`.
- El contenedor monta el directorio de datos del servidor en la misma ruta dentro y fuera, para que las rutas de los compose se resuelvan igual que en el host.
- Se publica solo en `localhost` y se accede por `tailscale serve`, sin login propio.
- Si una versión nueva no arranca, se vuelve a la anterior por SSH cambiando la versión en el compose.
