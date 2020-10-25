<?php
$page = basename($_SERVER['PHP_SELF']);
?>

<style>
    .underlined{
    text-decoration: underline;
    }
</style>
          
          <div class="card">
            <div class="card-body" align="center">
              <img src="img/delegao.jpg" alt="Delegao avatar" class="avatar mb-4">
              
              <h1>Rubén<span class="font-weight-light"> M.</span></h1>
              <h5><span class="font-weight-light">(aka <i>delegao</i>)</span></h5>
              <p class="text-white-75 font-weight-light mb-2">Climate activist. Archivist. Against censorship. Student.</p>
              <p class="font-weight-light"><a href="keys"><i class="fas fa-key"></i> gpg keys</a> - <a href="guestbook"><i class="fas fa-address-book"></i> guestbook</a> - <a href="donate"><i class="fas fa-hand-holding-usd"></i> donate</a></p>
              <hr>
              <h5 class="card-title">Navigation</h5>
              <h6 class="card-subtitle mb-2 text-muted">and a quick link to the services that I run...</h6>
              <div id="menu">
                <a href="about-me" class="h4 display-4 text-white <?php if($page == 'about-me.php'){ echo 'underlined';}?>">about me</a><br>
                <a href="projects" class="h4 display-4 text-white <?php if($page == 'projects.php'){ echo 'underlined';}?>">projects</a><br>
                <a href="music" class="h4 display-4 text-white <?php if($page == 'music.php'){ echo 'underlined';}?>">music</a><br>
                <a href="games" class="h4 display-4 text-white <?php if($page == 'games.php'){ echo 'underlined';}?>">games</a><br>
                <a href="contact" class="h4 display-4 text-white <?php if($page == 'contact.php'){ echo 'underlined';}?>">contact</a>
                  <hr>
                <p><a target="_blank" href="https://safe.delegao.moe/">safe</a> - <a target="_blank" href="https://lainsafe.delegao.moe/">lainsafe</a> - <a target="_blank" href="https://paste.delegao.moe/">paste</a> - <a target="_blank" href="https://ip.delegao.moe/">ip checker</a> - <a target="_blank" href="https://meet.delegao.moe/">meet</a></p>
              </div>
            </div>
          </div>