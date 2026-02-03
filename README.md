# Remote Process Monitor

Este proyecto consiste en un sistema de monitoreo y comunicación distribuido que permite la gestión de procesos en múltiples máquinas a través de una red local. El sistema está compuesto por una arquitectura cliente-servidor con un middleware de descubrimiento dinámico.

## Características

- Gestión de procesos: Listado detallado, inicio y detención de procesos remotos.
- Monitoreo en tiempo real: Visualización de consumo de CPU y memoria por proceso.
- Arquitectura Distribuida: Interacción con múltiples nodos servidores simultáneamente.
- Descubrimiento Automático: Implementación de UDP Broadcast para localizar servidores activos sin configuración manual de IPs.
- Interfaz de Usuario: TUI (Terminal User Interface) interactiva construida con el framework Bubble Tea.
- Ordenamiento Dinámico: Capacidad de ordenar procesos por PID, Nombre, CPU o Memoria.

## Tecnologías Utilizadas

- Lenguaje de Programación: Go (Golang).
- Comunicación de Red: Sockets TCP/IP para comandos y UDP para descubrimiento.
- Formato de Datos: JSON para la serialización de mensajes.
- Librerías Principales:
  - gopsutil: Para la recolección de métricas del sistema operativo.
  - Bubble Tea: Para la gestión de la interfaz de terminal.
  - Lip Gloss: Para el estilizado de componentes en la terminal.

## Estructura del Proyecto

- cmd/server: Punto de entrada para la aplicación del servidor (agente de monitoreo).
- cmd/client: Punto de entrada para la aplicación cliente (interfaz de control).
- internal/process: Lógica interna para la interacción con el sistema operativo y gestión de procesos.
- internal/protocol: Definición de las estructuras de datos y contratos de comunicación.

## Requisitos

- Go 1.18 o superior instalado.
- Sistema Operativo Linux (Recomendado), macOS o Windows.
- Conexión a red local con permisos para tráfico UDP Broadcast.

## Instalación

1. Clonar el repositorio:
   git clone <url-del-repositorio>
   cd remote-monitor

2. Descargar dependencias:
   go mod download

## Uso

### Ejecución del Servidor

El servidor debe ejecutarse en cada máquina que se desee monitorear.

go run cmd/server/main.go

El servidor comenzará a escuchar peticiones TCP en el puerto 8080 y emitirá anuncios UDP en el puerto 9999.

### Ejecución del Cliente

El cliente se ejecuta en la máquina desde la cual se realizará el monitoreo.

go run cmd/client/main.go

## Controles de la Interfaz (TUI)

- s: Conectar al primer servidor descubierto en la red.
- k: Detener (Kill) el proceso seleccionado en la tabla.
- 1: Ordenar procesos por PID.
- 2: Ordenar procesos por Nombre.
- 3: Ordenar procesos por consumo de CPU.
- 4: Ordenar procesos por consumo de Memoria.
- Flechas Arriba/Abajo: Navegar por la lista de procesos.
- q o Ctrl+C: Salir de la aplicación.

## Detalles Técnicos de Red

- Protocolo de Descubrimiento: Los servidores envían un JSON vía UDP al puerto 9999 cada 2 segundos.
- Protocolo de Comando: El cliente establece una conexión TCP al puerto 8080 del servidor seleccionado para intercambiar mensajes JSON de tipo CommandRequest y Response.
