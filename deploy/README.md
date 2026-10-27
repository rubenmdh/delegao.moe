# Despliegue de delegao.moe

Para la **Orange Pi Zero de 512 MB existente**, con nginx, varias aplicaciones
PHP y una base MySQL externa, usa [sqlite-migration.md](sqlite-migration.md).
El backend SQLite evita instalar un servidor de base de datos en la placa.

[orange-pi.md](orange-pi.md) conserva la alternativa MySQL/MariaDB local.

- `build.ps1`: compilación cruzada ARMv7 o ARM64 desde PowerShell, sin CGO.
- `delegao.env.example`: SQLite local, puerto 18080 y límites iniciales.
- `delegao.service`: servicio systemd sin privilegios; no requiere MariaDB.
- `nginx.conf`: ejemplo de instalación nueva con HTTPS y proxy hacia Go:18080.
- `nginx-bootstrap.conf`: ACME HTTP para instalaciones nuevas sin certificado.
- `backup-sqlite.sh`: copia verificada de SQLite mediante su API de backup.
- `backup.sh`: alternativa para MySQL/MariaDB local.
- `mariadb-small.cnf`: perfil opcional para quien elige instalar MariaDB.

En una instalación existente conserva el virtual host y su terminación HTTPS;
no sustituyas nginx entero ni detengas PHP usado por otros dominios. Los archivos
no se han aplicado al servidor ni al DNS. El consumo real debe medirse en la placa.