# Despliegue de delegao.moe

La guía completa para la **Orange Pi Zero de 512 MB**, con el dominio
**delegao.moe**, está en [orange-pi.md](orange-pi.md).

- `build.ps1`: compilación cruzada ARMv7 o ARM64 desde PowerShell.
- `delegao.env.example`: configuración de la aplicación y límites iniciales.
- `delegao.service`: servicio systemd sin privilegios.
- `mariadb-small.cnf`: punto de partida para MySQL/MariaDB local de poco consumo.
- `nginx-bootstrap.conf`: emisión inicial del certificado mediante HTTP/ACME.
- `nginx.conf`: HTTPS y proxy hacia el servidor mixto Go.
- `backup.sh`: copia consistente de MySQL, ejecutada localmente como root.

Los archivos están preparados para instalarse siguiendo la guía; no se han
aplicado al servidor ni al DNS. El consumo real debe verificarse en la placa.
