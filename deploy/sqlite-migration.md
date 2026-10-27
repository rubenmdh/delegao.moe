# Migrar delegao.moe de PHP/MySQL externo a Go/SQLite

Esta guía corresponde a la instalación existente: **Orange Pi Zero ARMv7 de
32 bits, 512 MB, Armbian Bookworm, nginx y varias aplicaciones PHP**. La base
PHP está en `lu-shared01.dapanel.net`, se llama `rubenmdh_delegaodb` y usa el
usuario `rubenmdh_remoteuser`. Estos tres datos no incluyen la contraseña.

El objetivo es **nginx → un proceso Go → SQLite local**, conservando el panel
web y el guestbook dinámicos. El resto del sitio se genera en disco con dos
workers. No requiere instalar MariaDB server en la placa. PHP 8.4 y 8.5 deben
seguir funcionando para los otros dominios. Otro virtual host usa 8080; el
backend Go utilizará **127.0.0.1:18080**, si ese puerto está libre.

La terminación HTTPS actual aún debe confirmarse. Conserva la configuración
que ya sirve `delegao.moe`, sus certificados y cualquier proxy externo. No
copies `nginx.conf` completo sobre la configuración existente: ese archivo es
un ejemplo para una instalación nueva con certificados en las rutas indicadas.

## 1. Copias y comprobaciones previas

Ya se han guardado copias de nginx y del sitio PHP en
`/root/delegao-migration/`, con permisos privados. Compruébalas:

```sh
ls -lh /root/delegao-migration/
ss -ltnp | grep -E ':(80|443|8080|18080)[[:space:]]'
readlink -f /etc/nginx/sites-enabled/delegao.moe.conf
```

Identifica cómo se sirve HTTPS antes del cambio de tráfico: nginx local,
Cloudflare u otro proxy. En el virtual host conserva cualquier ruta ACME,
redirección o ajuste que necesiten los certificados actuales.

Instala las herramientas de respaldo/importación, sin servidor MariaDB:

```sh
apt update
apt install --no-install-recommends sqlite3 ca-certificates mariadb-client
```

`mariadb-client` solo es necesario para el dump SQL de seguridad; el importador
Go lee MySQL directamente. Una vez terminada la migración, la aplicación solo
necesita su ejecutable y los archivos de estado locales.

Haz un dump privado del origen. Se pide la contraseña de MySQL en el terminal;
no va en el comando ni en el nombre del archivo:

```sh
umask 077
source_dump="/root/delegao-migration/mysql-php-$(date -u +%Y%m%dT%H%M%SZ).sql"
mariadb-dump --host=lu-shared01.dapanel.net \
  --user=rubenmdh_remoteuser --password \
  --ssl --ssl-verify-server-cert \
  --single-transaction --quick --skip-lock-tables \
  rubenmdh_delegaodb > "$source_dump"
```

Comprueba que terminó sin error antes de continuar:

```sh
test -s "$source_dump" && ls -lh "$source_dump"
sha256sum "$source_dump"
```

Un archivo no vacío no demuestra por sí solo que el dump haya terminado:
revisa también el código de salida y cualquier error del cliente. Conserva una
copia fuera de la SD. Si el proveedor usa una CA privada, obtén su certificado
y añade `--ssl-ca=/ruta/ca-del-proveedor.pem`. No desactives la verificación TLS
para continuar tras un error; resuelve primero la conexión con el proveedor.

## 2. Compila y copia el binario ARMv7

En el ordenador de desarrollo, dentro del repositorio:

```powershell
./deploy/build.ps1 -Target armv7 -Go "$PWD/.tools/go/bin/go.exe"
scp ./dist/delegao-linux-armv7 root@IP_ORANGE_PI:~/delegao-new
scp -r ./deploy root@IP_ORANGE_PI:~/deploy-delegao
```

Sustituye `IP_ORANGE_PI` por tu dirección SSH real. Si utilizas un usuario sin
privilegios, copia allí los archivos y ejecuta los pasos de instalación con
`sudo`. SQLite está incluido en el binario, sin CGO; no se compila en la placa.
Comprueba que la rama incluye el soporte SQLite antes de usar el binario.

## 3. Instala el servicio y el entorno

En la Orange Pi, como root:

```sh
id delegao >/dev/null 2>&1 || useradd --system --user-group \
  --home-dir /var/lib/delegao --shell /usr/sbin/nologin delegao
install -o root -g root -m 0755 ~/delegao-new /usr/local/bin/delegao
install -d -o delegao -g delegao -m 0750 /var/lib/delegao /var/lib/delegao/public
install -o root -g delegao -m 0640 \
  ~/deploy-delegao/delegao.env.example /etc/delegao.env
```

Revisa `/etc/delegao.env`:

```sh
ADDR=127.0.0.1:18080
DATABASE_DRIVER=sqlite
DATABASE_DSN=/var/lib/delegao/site.db
PUBLIC_DIR=/var/lib/delegao/public
BUILD_WORKERS=2
DB_MAX_OPEN_CONNS=4
DB_MAX_IDLE_CONNS=1
COOKIE_SECURE=true
TRUSTED_PROXIES=127.0.0.1/32
GUESTBOOK_ANSWER=Ravioli
GOMEMLIMIT=64MiB
GOMAXPROCS=2
```

El archivo no necesita credenciales MySQL para el funcionamiento diario.
`COOKIE_SECURE=true` presupone que el navegador accede por HTTPS, aunque nginx
hable HTTP con Go en loopback. No lo cambies a `false` en el servicio público.

## 4. Importación de prueba sin cambiar nginx

Puedes ensayar la importación en un archivo independiente `import-check.db`,
mientras la web PHP sigue sirviendo tráfico. Esta copia es de prueba y no se
convertirá en la base final; puede quedar desactualizada si llegan comentarios.

Primero crea su esquema:

```sh
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; export DATABASE_DSN=/var/lib/delegao/import-check.db; exec /usr/local/bin/delegao migrate'
```

Importa en esa base vacía:

```sh
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; export DATABASE_DSN=/var/lib/delegao/import-check.db; exec /usr/local/bin/delegao import-mysql lu-shared01.dapanel.net rubenmdh_delegaodb rubenmdh_remoteuser'
```

La contraseña se solicita sin eco. Usa la contraseña actual de la base PHP;
no se añade al historial, a argumentos del proceso ni a registros. La conexión
MySQL exige TLS verificado. Si el proveedor usa una CA privada, añade al entorno
`MYSQL_SOURCE_CA=/ruta/ca-del-proveedor.pem`, legible por el usuario `delegao`.

Si has aceptado explícitamente un certificado autofirmado, incluso sin nombre
de dominio, puedes usar `MYSQL_SOURCE_SHA256` con su huella SHA-256 completa
(con o sin dos puntos). Añade `export MYSQL_SOURCE_SHA256="HUELLA_ACEPTADA"`
dentro del `sh -c` anterior, antes de `exec`. Esta opción exige TLS y comprueba
la huella exacta y la vigencia del certificado en cada conexión, sustituyendo
la comprobación de CA y nombre. Un certificado renovado requiere aceptar su
nueva huella. No combines esta opción con `MYSQL_SOURCE_CA`.

El importador solo lee el origen, en una instantánea consistente, y escribe el
destino en una transacción. Conserva los IDs, fechas, etiquetas, borradores,
entradas del guestbook y hashes de contraseña. Normaliza una sola vez el texto
que guardaba PHP. Si el origen ya tenía la migración Go de entidades, no vuelve
a decodificarlo. Rechaza un destino que ya tenga contenido; no sobrescribe datos.

Para revisar los recuentos de la copia:

```sh
sqlite3 -readonly /var/lib/delegao/import-check.db \
  'SELECT "posts",COUNT(*) FROM news UNION ALL SELECT "guestbook",COUNT(*) FROM guestbook UNION ALL SELECT "users",COUNT(*) FROM users;'
```

Si quieres probar el panel, crea primero sus páginas de prueba y arranca una
instancia temporal en otro puerto, con su propio directorio de publicación:

```sh
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; export DATABASE_DSN=/var/lib/delegao/import-check.db PUBLIC_DIR=/var/lib/delegao/check-public ADDR=127.0.0.1:18081 COOKIE_SECURE=false; exec /usr/local/bin/delegao'
```

La instancia queda en primer plano. Desde tu ordenador abre un túnel:

```powershell
ssh -N -L 18081:127.0.0.1:18081 root@IP_ORANGE_PI
```

Visita <http://127.0.0.1:18081/login>, entra con tu usuario existente y comprueba
entradas, borradores y guestbook. Los cambios de esta copia no afectan al origen.
El uso de cookies HTTP se limita a esta prueba local dentro del túnel. Detén el
servidor de prueba con Ctrl+C cuando termines.

## 5. Copia final y cambio de tráfico

Haz esta fase después de revisar el virtual host real y la terminación HTTPS.
Antes de la importación final, impide nuevas escrituras en el sitio PHP de
`delegao.moe`. La opción más simple es una breve ventana de mantenimiento solo
para ese dominio: devolver 503 temporalmente en su bloque `server`. No detengas
nginx ni los servicios PHP compartidos por las otras webs. Comprueba siempre
`nginx -t` antes de recargar.

Con las escrituras detenidas, repite el dump de seguridad del paso 1. Usa ahora
el archivo final nuevo `/var/lib/delegao/site.db`, todavía vacío:

```sh
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao migrate'
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao import-mysql lu-shared01.dapanel.net rubenmdh_delegaodb rubenmdh_remoteuser'
sudo -u delegao sh -c 'set -a; . /etc/delegao.env; set +a; exec /usr/local/bin/delegao build'
```

No ejecutes `create-admin`: los usuarios y sus hashes ya se han importado.
Comprueba los recuentos con la misma consulta SQLite del paso anterior,
sustituyendo `import-check.db` por `site.db`, y contrástalos con el resumen del
importador. Si algo falla, conserva la web PHP y el MySQL originales; no fuerces
una nueva importación sobre un destino parcialmente usado.

Instala y arranca el servicio:

```sh
install -o root -g root -m 0644 ~/deploy-delegao/delegao.service \
  /etc/systemd/system/delegao.service
systemctl daemon-reload
systemctl enable --now delegao
systemctl --no-pager --full status delegao
curl -I http://127.0.0.1:18080/
```

Go reconstruye al arrancar. La respuesta pública debe tener ETag y
`Cache-Control: public, no-cache`. Una visita a `/guestbook` o `/login` debe
usar `Cache-Control: no-store`.

En el virtual host de **delegao.moe solamente**, sustituye el despacho PHP por
un proxy a Go. Este bloque es el contenido de su `location /`, no un archivo
nginx completo:

```nginx
location / {
    proxy_pass http://127.0.0.1:18080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_connect_timeout 5s;
    proxy_read_timeout 180s;
    proxy_send_timeout 180s;
}
```

Retira también de este virtual host las reglas FastCGI y reescrituras a `.php`
que interceptarían las rutas antiguas antes de llegar a Go. Conserva los ajustes
TLS/ACME vigentes y configura `client_max_body_size 1m`. No cambies los virtual
hosts de `po.mt`, `basicqr`, `ipv6.sx`, `n2u.eu` ni otros servicios. Si hay un
proxy HTTPS externo, valida su configuración antes de activar este bloque;
no deduzcas la URL pública a partir del HTTP interno de loopback.

```sh
nginx -t && systemctl reload nginx
```

Comprueba el dominio por HTTPS, el acceso, un borrador, publicación, edición,
despublicación y guestbook. Los usuarios conservarán su contraseña, pero tendrán
que iniciar sesión de nuevo.

## 6. Copias SQLite, actualizaciones y vuelta atrás

Instala el helper de backup:

```sh
install -o root -g root -m 0750 ~/deploy-delegao/backup-sqlite.sh \
  /usr/local/sbin/delegao-backup-sqlite
set -a
. /etc/delegao.env
set +a
/usr/local/sbin/delegao-backup-sqlite "$DATABASE_DSN"
```

Usa la API online de SQLite y verifica la copia con `quick_check`. Se puede
usar mientras Go está funcionando. No copies únicamente el `.db` en caliente:
con WAL activo puede haber cambios confirmados en el archivo `-wal`. Guarda
las copias fuera de la SD y conserva una copia protegida de `/etc/delegao.env`.
Pasa siempre la ruta de `DATABASE_DSN`: durante el corte se puede haber elegido
un nombre nuevo, como `site-final.db`, en lugar del `site.db` del ejemplo.

Para restaurar una copia verificada, detén Go, conserva el conjunto actual
`.db`, `-wal` y `-shm` si existe, restaura la copia con propietario `delegao` y
reconstruye antes de arrancar. No mezcles un `.db` restaurado con el WAL de otra
base. Mantén un único proceso controlando el directorio de publicación.

### Acentos o emojis con doble codificación

Si aparecen secuencias como `RubÃ©n` o `â€™`, revisa los bytes y la codificación
del origen antes de atribuirlo a SQLite. La conexión del importador usa
`utf8mb4`, pero MySQL convierte el texto según el charset declarado de cada
columna. Si ese charset no corresponde a los bytes que guardaba PHP, puede
devolver texto ya mal interpretado. El importador conserva ese texto.

Para una copia ya importada hay una herramienta de reparación. Desde el PC:

```powershell
scp ./deploy/repair-encoding.py root@IP_ORANGE_PI:/root/repair-encoding.py
```

Instálala y revisa su propuesta desde la Orange Pi:

```sh
install -d -o root -g root -m 0755 /usr/local/lib/delegao
install -o root -g root -m 0755 /root/repair-encoding.py \
  /usr/local/lib/delegao/repair-encoding.py
runuser -u delegao -- sh -c 'set -a; . /etc/delegao.env; set +a; exec python3 /usr/local/lib/delegao/repair-encoding.py "$DATABASE_DSN"'
```

Sin `--apply` solo muestra ejemplos y recuentos. La herramienta reconoce
secuencias UTF-8 reversibles interpretadas como Latin-1 o Windows-1252, también
en campos donde parte del texto ya está bien. No usa sustitución de caracteres
inválidos ni convierte el campo completo a una codificación antigua. Los
ejemplos de mojibake escritos intencionadamente siguen siendo ambiguos: revisa
la propuesta antes de aplicarla.

Si los cambios propuestos corresponden al texto que quieres recuperar:

```sh
systemctl stop delegao
runuser -u delegao -- sh -c 'set -a; . /etc/delegao.env; set +a; exec python3 /usr/local/lib/delegao/repair-encoding.py "$DATABASE_DSN" --apply'
systemctl start delegao
journalctl -u delegao -n 20 --no-pager
```

Antes de cambiar datos crea una copia consistente mediante la API online de
SQLite y verifica `quick_check`. La ruta aparece en la salida; por defecto está
en `encoding-backups/` junto a la base. Las actualizaciones se confirman en una
sola transacción. Si falla una actualización, se revierten todas. Se conservan
usuarios, hashes, IDs, fechas, estados y comentarios nuevos. No se decodifican
entidades HTML ni se modifica MySQL. El arranque de Go regenera las páginas.
No hace falta volver a importar la base ni sustituir el ejecutable para usarla.

Para volver a la web PHP durante el cambio inicial, recupera su virtual host
original y recarga nginx después de `nginx -t`. La importación no ha modificado
el MySQL externo ni los archivos PHP. Si ya se ha utilizado Go en producción,
primero respalda SQLite: las nuevas publicaciones o entradas del guestbook no
estarán en el MySQL antiguo y necesitan una migración inversa para conservarse.

Mide la placa después del arranque:

```sh
free -h
ps -eo pid,comm,rss --sort=-rss | head
systemctl show delegao -p MemoryCurrent -p MemoryPeak -p NRestarts
df -h /var/lib/delegao
```

Los límites incluidos son valores iniciales, no mediciones en esta placa.
SQLite no añade un daemon MySQL local, pero comparte CPU, RAM y la SD con PHP,
nginx y los otros sitios. Ante presión de memoria baja `BUILD_WORKERS` y
`GOMAXPROCS` a 1. Conserva la durabilidad y las copias externas.

Referencias: [SQLite WAL](https://sqlite.org/wal.html),
[SQLite online backup](https://sqlite.org/backup.html),
[Go memory limit](https://go.dev/doc/gc-guide),
[nginx proxy](https://nginx.org/en/docs/http/ngx_http_proxy_module.html).
