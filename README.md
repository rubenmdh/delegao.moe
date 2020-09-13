# delegao.moe website repository
<img src="https://lainsafe.delegao.moe/files/160001407551628.png" alt="delegao.moe logo" width="40%"/>
This repository contains the PHP script running on my personal website, delegao.moe.
It features a microblogging system and a web guestbook.

Built with PHP and the Bootstrap 4 library, it is a clean and minimalistic personal website script.

## Features
- Blogging system
- Drafts
- Guestbook
- Admin panel

## To-do list
- Migrate from TinyMCE to Markdown
- Tags
- Categories
- Guestbook entries management
- User management interface
- A more feature rich administrator management interface

## Installation
1. Download the source code on your webroot
2. Unzip it
3. Create a MySQL/MariaDB database
4. Go to https://yourwebsite.com/install.php with your web browser and follow the steps
5. Set up your webserver rewrite rules (more information below)
   - With Apache, copying the .htaccess file on your webroot should suffice
   - With nginx, use the rules below
 ```
location / {
     if (!-e $request_filename){
      rewrite ^(.*)$ /$1.php;
     }
 }
```

## Author
delegao.moe is solely mantained by me, released under the GPLv3 license

delegao.moe contains external software:
  - [Bootstrap 4](https://github.com/twbs/bootstrap)
  - [TinyMCE](https://github.com/tinymce/tinymce/)
