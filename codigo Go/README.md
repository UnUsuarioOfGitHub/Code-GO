# Backend de pedidos en Go

Servicio independiente en Go para `POST /api/orders`, con su propia base de datos, migracion, Docker Compose y plan JMeter.

## Requisitos

- Go 1.23
- Docker con Docker Compose
- Apache JMeter 5.6.3 o posterior

## Arranque

Desde esta carpeta:

```powershell
Copy-Item .env.example .env
docker compose up --build -d
docker compose ps
```

La API queda disponible en `http://localhost:8080` y PostgreSQL en el puerto `5433` del equipo. La API se conecta a `db:5432` dentro de la red privada de Compose. La base de datos tiene un volumen independiente llamado `go_dbdata`.

El esquema se crea al iniciar un volumen nuevo. El SQL de inicializacion no se vuelve a ejecutar si el volumen ya existe.

## Endpoint

`POST /api/orders`, con `Content-Type: application/json`:

```json
{
  "customerId": "C123",
  "items": [
    { "productId": "P1", "quantity": 2, "price": 10.5 },
    { "productId": "P2", "quantity": 1, "price": 5.0 }
  ]
}
```

La respuesta exitosa es `201 Created`. `itemsCount` es la suma de las cantidades de las lineas. Los importes se representan como centavos enteros durante el calculo y se escriben como `NUMERIC(10,2)`. Se rechazan importes negativos y precios con mas de dos decimales no nulos. El ID tiene el formato `ORD-` seguido de un ULID.

Prueba manual desde PowerShell:

```powershell
curl.exe -i http://localhost:8080/api/orders -H "Content-Type: application/json" -d "{\"customerId\":\"C123\",\"items\":[{\"productId\":\"P1\",\"quantity\":2,\"price\":10.5},{\"productId\":\"P2\",\"quantity\":1,\"price\":5.0}]}"
```

`GET /healthz` comprueba que el proceso responde. `GET /readyz` comprueba ademas la conexion con PostgreSQL. Las respuestas JSON no usan gzip y el servicio no aplica limitacion de solicitudes.

## Configuracion

Los valores iniciales estan en `.env.example`. Compose los carga desde `.env`. El pool inicial usa maximo 20 y minimo 5 conexiones. `DefaultQueryExecMode` se fija en `CacheStatement`, modo predeterminado de pgx que puede afectar memoria y latencia.

## Pruebas de carga con JMeter

Abre `tests/load/orders.jmx` desde esta carpeta. El plan usa datos CSV mas UUID por solicitud para variar clientes y productos. Para evitar el costo de la interfaz grafica:

```text
jmeter -n -t tests/load/orders.jmx -l results-10.jtl -e -o report-10 -JAPI_PORT=8080 -JUSERS=10 -JRAMP_SECONDS=10 -JDURATION_SECONDS=120
```

El informe HTML incluye percentiles como P90 y P95. Usa una carpeta de salida distinta o elimina el informe anterior antes de repetir la misma corrida.

Repite con 50 usuarios y rampa de 30 segundos, y con 100 usuarios y rampa de 60 segundos. Cada escenario estable dura 120 segundos. Para estres, ejecuta corridas separadas aumentando `USERS` de 20 en 20, con rampa de 30 segundos y duracion de 60 segundos por nivel. Deten la ejecucion con `Ctrl+C` cuando los errores superen sostenidamente el 5 por ciento o el P95 exceda el umbral que acuerde el grupo.

El PDF presenta dos conjuntos de carga: las paginas 3 y 4 piden respectivamente 20/70/150 y 10/50/100 usuarios. Este plan usa 10/50/100 como valores predeterminados; ambos conjuntos se pueden ejecutar pasando `USERS` y `RAMP_SECONDS`. Confirma con el grupo cual usaran los cuatro stacks.

El plan incluye Summary Report y Aggregate Report. Registra promedio, P90/P95, throughput y porcentaje de errores. Durante cada escenario, toma capturas de:

- Summary Report y Aggregate Report.
- JMeter ejecutandose bajo carga.
- `docker ps` con los contenedores levantados.
- `docker stats go-api-orders go-db-orders`.
- `htop` durante cada escenario.

Ejecuta JMeter en un equipo separado cuando sea posible; de lo contrario, registra sus recursos para considerar su efecto.

## Desarrollo local

Con PostgreSQL disponible y las variables de entorno configuradas:

```text
go run ./cmd/api
go test ./...
```

El `Makefile` incluye `build`, `run`, `test`, `compose-up`, `compose-down` y `jmeter`.
