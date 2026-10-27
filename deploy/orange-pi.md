# Despliegue en Orange Pi Zero de 512 MB: delegao.moe

La configuración recomendada es **nginx + un proceso Go + MariaDB local**,
con systemd. No necesita PHP, Node.js, Hugo ni Docker en la placa. Compila en
otro ordenador y copia únicamente el ejecutable y los archivos de `deploy/`.

MySQL conserva las entradas editables del blog. Publicar, modificar, despublicar o borrar
contenido reconstruye el sitio público con **dos workers limitados** y cambia
de generación completa. Las lecturas públicas usan HTML en disco; el panel y
el guestbook siguen siendo dinámicos. nginx envía todas las rutas a Go para
respetar el manifiesto y las restricciones de publicación: **no publiques
directamente los directorios de generaciones con `root`, alias o un CDN**.

Las páginas informativas originales siguen siendo plantillas incluidas en el
binario; cambiarlas requiere editar la plantilla, recompilar y reconstruir.
El editor web gestiona las entradas del blog, no es un CMS de todas las páginas.

## 1. Comprueba la placa y el sistema

En la Orange Pi:

```sh
uname -m
getconf LONG_BIT
free -h
cat /etc/os-release
```

La **Zero original**, con Allwinner H2/H2+ y Cortex-A7, usa ARM de 32 bits:
normalmente `armv7l`, para el que necesitas `GOARCH=arm GOARM=7`. No uses un
binario `arm64` por el mero hecho de que la placa se llame Orange Pi. Algunos
modelos Zero posteriores son distintos; con un sistema `aarch64` de 64 bits
elige `GOARCH=arm64`. Importa también la arquitectura del sistema instalado.
[Manual del fabricante](https://orangepi.net/wp-content/uploads/2022/11/OrangePi-Zero_H2-User-Manual_v0.9.1.pdf),
[Go sobre ARM](https://go.dev/wiki/GoArm).

Usa una imagen mínima Debian/Armbian que siga recibiendo actualizaciones y
esté soportada para tu modelo, con systemd y MariaDB 10.11 o posterior.
Esta guía usa rutas y paquetes de Debian; no está probada en la placa física.
No reutilices una imagen antigua del fabricante expuesta a Internet.

## 2. Compila fuera de la placa

Desde la raíz del repositorio, en PowerShell con Go 1.26 o posterior:

```powershell
./deploy/build.ps1 -Target armv7
```

En este checkout también puedes usar el SDK portátil empleado para verificar
el proyecto:

```powershell
./deploy/build.ps1 -Target armv7 -Go "$PWD/.tools/go/bin/go.exe"
```

Para un sistema ARM64 usa `-Target arm64`. El helper restaura las variables de
entorno al terminar y muestra el SHA-256 del ejecutable en `dist/`.

La compilación equivalente en Linux/macOS es:

```sh
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -ldflags='-s -w' -o dist/delegao-linux-armv7 .
```

Copia el ejecutable y `deploy/` a una carpeta de tu usuario SSH en la placa
(sustituye `USUARIO@IP_ORANGE_PI` por tu acceso real):

```powershell
scp ./dist/delegao-linux-armv7 USUARIO@IP_ORANGE_PI:~/delegao-new
scp -r ./deploy USUARIO@IP_ORANGE_PI:~/deploy-delegao
```

## 3. Paquetes, cuenta y directorios

En la placa:

```sh
sudo apt update
sudo apt install --no-install-recommends nginx mariadb-server certbot ca-certificates
id delegao >/dev/null 2>&1 || sudo useradd --system --user-group \
  --home-dir /var/lib/delegao --shell /usr/sbin/nologin delegao
sudo install -o root -g root -m 0755 ~/delegao-new /usr/local/bin/delegao
sudo install -d -o delegao -g delegao -m 0750 /var/lib/delegao /var/lib/delegao/public
sudo install -o root -g delegao -m 0640 \
  ~/deploy-delegao/delegao.env.example /etc/delegao.env
sudoedit /etc/delegao.env
```

Sustituye `REPLACE_WITH_DB_PASSWORD`. Utiliza una contraseña aleatoria que no
contenga apóstrofos ni saltos de línea para mantener válidas las comillas del
ejemplo. El DSN usa `/run/mysqld/mysqld.sock`; comprueba la ruta con
`sudo mariadb -Nse 'SELECT @@socket'` y corrígela si es distinta.

`/etc/delegao.env` contiene el secreto de la base de datos. systemd lo lee,
pero el ejecutable no carga archivos `.env` automáticamente. No guardes este
archivo en Git ni lo copies al directorio público.

## 4. MariaDB y migración

El perfil incluido reserva 64 MiB para el buffer principal de InnoDB y reduce
otros caches y conexiones. **No limita toda la memoria de MariaDB a 64 MiB**.
Desactiva TCP para esta base local; si esa instancia da servicio a otras
aplicaciones remotas, adapta el perfil antes de instalarlo.

```sh
sudo install -o root -g root -m 0644 ~/deploy-delegao/mariadb-small.cnf \
  /etc/mysql/mariadb.conf.d/60-delegao-small.cnf
sudo systemctl restart mariadb
sudo systemctl --no-pager --full status mariadb
sudo mariadb
```

En el prompt SQL, sustituyendo la contraseña por la de `/etc/delegao.env`:

```sql
CREATE DATABASE delegao CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'delegao'@'localhost' IDENTIFIED BY 'REPLACE_WITH_DB_PASSWORD';
GRANT SELECT, INSERT, UPDATE, DELETE ON delegao.* TO 'delegao'@'localhost';
GRANT CREATE, ALTER, INDEX ON delegao.* TO 'delegao'@'localhost';
QUIT;
```

Si migras la instalación PHP, **detén sus escrituras y haz una copia antes**.
No ejecutes `CREATE DATABASE` sobre una base ya existente. Restaura su dump en
la base elegida antes de ejecutar la migración Go:

```sh
sudo mariadb delegao < /ruta/al/backup-php.sql
```

Migra, crea el administrador solo si todavía no tienes uno y genera el sitio.
Las comillas del archivo de ejemplo permiten cargarlo en esta consola:

```sh
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao migrate'
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao create-admin delegao'
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao build'
```

La contraseña administrativa se solicita sin eco. Los hashes bcrypt de PHP
existentes funcionan y no necesitas crear otro usuario para migrarlos.
Retira después los permisos de migración; el servicio diario solo necesita
lectura y escritura de datos:

```sh
sudo mariadb
```

```sql
REVOKE CREATE, ALTER, INDEX ON delegao.* FROM 'delegao'@'localhost';
QUIT;
```

En futuras actualizaciones con cambios de esquema, concede esos permisos
temporalmente durante la parada y la migración, y revócalos de nuevo.

## 5. Arranca Go con systemd

```sh
sudo install -o root -g root -m 0644 ~/deploy-delegao/delegao.service \
  /etc/systemd/system/delegao.service
sudo systemctl daemon-reload
sudo systemctl enable --now delegao
sudo systemctl --no-pager --full status delegao
curl -I http://127.0.0.1:8080/
```

Go escucha únicamente en loopback. El directorio de estado es escribible para
`delegao`; el ejecutable y la configuración permanecen bajo control de root.
El servicio usa un objetivo de memoria de Go de 64 MiB, `MemoryHigh=96M` y un
límite duro de 160 MiB. El objetivo de Go no es un límite del RSS total.
Los límites de systemd dependen de que el kernel y los cgroups los soporten.

## 6. DNS y HTTPS para delegao.moe

Apunta el registro **A de `delegao.moe`** a tu IPv4 pública. Publica un registro
AAAA solo si dispones de IPv6 pública que llegue a esta placa. En el router,
reenvía TCP 80 y 443 a la Orange Pi. No publiques 8080 ni MariaDB. Bajo CGNAT
necesitarás una dirección entrante, un proxy público o un túnel apropiado;
los cambios DNS por sí solos no lo solucionan.

Primero instala el bloque HTTP de ACME, que no necesita certificados previos:

```sh
sudo install -d -o root -g root -m 0755 /var/lib/letsencrypt
sudo install -o root -g root -m 0644 ~/deploy-delegao/nginx-bootstrap.conf \
  /etc/nginx/sites-available/delegao.moe
sudo ln -s /etc/nginx/sites-available/delegao.moe /etc/nginx/sites-enabled/delegao.moe
sudo nginx -t
sudo systemctl reload nginx
sudo certbot certonly --webroot -w /var/lib/letsencrypt -d delegao.moe
```

Si ya existe el enlace, no lo vuelvas a crear. Revisa previamente cualquier
bloque nginx existente para ese mismo dominio. Completa los pasos interactivos
de Certbot. Este bloque devuelve 503 fuera de ACME durante la preparación;
no se usa para acceder al administrador por HTTP.

Una vez emitido el certificado:

```sh
sudo install -o root -g root -m 0644 ~/deploy-delegao/nginx.conf \
  /etc/nginx/sites-available/delegao.moe
sudo nginx -t
sudo systemctl reload nginx
sudo install -d -m 0755 /etc/letsencrypt/renewal-hooks/deploy
printf '#!/bin/sh\nnginx -t && systemctl reload nginx\n' | \
  sudo tee /etc/letsencrypt/renewal-hooks/deploy/reload-nginx >/dev/null
sudo chmod 0755 /etc/letsencrypt/renewal-hooks/deploy/reload-nginx
sudo systemctl enable --now certbot.timer
sudo certbot renew --dry-run
```

El timer corresponde al paquete Debian; comprueba su existencia con
`systemctl list-timers certbot.timer`. Mantén `COOKIE_SECURE=true` y
`TRUSTED_PROXIES=127.0.0.1/32`: nginx preserva Host y sobrescribe
X-Forwarded-For para evitar que lo falsee un cliente.

## 7. Verifica una publicación real

Abre `https://delegao.moe/login`. Guarda un borrador, comprueba su vista previa,
publícalo y abre su URL desde una ventana sin sesión. Después edítalo y prueba
despublicarlo: la URL pública debe dejar de devolver su contenido.

```sh
curl -I https://delegao.moe/
curl -I https://delegao.moe/guestbook
sudo journalctl -u delegao -n 50 --no-pager
```

Las páginas generadas tienen ETag y requieren revalidación; no permiten que un
navegador mantenga indefinidamente una publicación retirada. El guestbook,
login y panel deben conservar `Cache-Control: no-store`.
Una publicación que no termine correctamente muestra el fallo en el panel;
el contenido guardado permanece en MySQL, pero el HTML público devuelve 503
hasta reconstruirse con éxito, evitando exponer versiones despublicadas.
Comprueba permisos, espacio y registro, y usa la reconstrucción del panel.
La reconstrucción en consola (`delegao build`) solo debe ejecutarse con el
servicio detenido: un único proceso debe controlar `PUBLIC_DIR`. Al arrancar,
el servidor intenta reconstruir de nuevo. Si la generación falla, mantiene
el panel dinámico disponible para reparar el problema y el HTML público
devuelve 503; si no puede conectar a MySQL, el proceso no arranca. El comando
`build` devuelve un código de error cuando no termina correctamente.

## Copias y actualizaciones

```sh
sudo install -o root -g root -m 0750 ~/deploy-delegao/backup.sh /usr/local/sbin/delegao-backup
sudo /usr/local/sbin/delegao-backup
```

Copia los `.sql.gz` fuera de la SD. El contenido fuente está en MySQL; las
generaciones son reproducibles y no sustituyen a esa copia. Conserva también
el ejecutable anterior y `/etc/delegao.env` en una copia protegida.
Para actualizar, sube el nuevo binario a `~/delegao-new`, comprueba su SHA-256,
haz la copia, para el servicio y sustituye el binario:

```sh
sudo systemctl stop delegao
sudo cp /usr/local/bin/delegao /usr/local/bin/delegao.previous
sudo install -o root -g root -m 0755 ~/delegao-new /usr/local/bin/delegao
# Si la versión añade migraciones, concede DDL temporalmente y ejecuta migrate.
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao build'
sudo systemctl start delegao
sudo systemctl --no-pager --full status delegao
```

Si la actualización falla y no hay cambios incompatibles de esquema, restaura
el ejecutable anterior y reconstruye con él antes de arrancar. Para volver a
PHP o revertir una migración incompatible, restaura conjuntamente su binario o
código y el backup de base de datos correspondiente. No publiques generaciones
viejas manualmente para saltarte un fallo de reconstrucción.

## Observa la memoria y la SD

```sh
free -h
ps -eo pid,comm,rss --sort=-rss | head
systemctl show delegao -p MemoryCurrent -p MemoryPeak -p NRestarts
sudo journalctl -k --grep='Out of memory\|oom-kill'
df -h /var/lib/delegao /var/lib/mysql
```

Dos workers son goroutines de generación, no dos procesos ni dos instancias
del servidor. Si hay presión de memoria baja `BUILD_WORKERS` y `GOMAXPROCS` a
1 antes de aumentar límites; observa también MariaDB. Los valores incluidos
son un punto de partida, **no una medición de consumo en tu placa**. Los caches
de la base, el sistema y nginx necesitan margen dentro de los 512 MB.
No compiles en la placa mientras sirve tráfico. Mantén espacio para dos
generaciones y el trabajo en curso. Limita la retención de journal y nginx
según el uso de la SD; conserva la durabilidad de InnoDB y copias externas.

Referencias: [memoria de MariaDB](https://mariadb.com/docs/server/ha-and-performance/mariadb-memory-allocation),
[buffer de InnoDB](https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-system-variables#innodb_buffer_pool_size),
[systemd.exec](https://www.freedesktop.org/software/systemd/man/256/systemd.exec.html),
[nginx proxy](https://nginx.org/en/docs/http/ngx_http_proxy_module.html),
[Certbot](https://certbot.eff.org/instructions?ws=nginx&os=pip).
