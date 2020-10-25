<?php
$page = basename($_SERVER['PHP_SELF']);
?>

<div class="container">
    <nav class="navbar navbar-expand-lg navbar-dark card mt-2 mb-5">
        <a class="navbar-brand" href="#">delegao.moe area 51</a>
        <button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarText" aria-controls="navbarText" aria-expanded="false" aria-label="Toggle navigation">
            <span class="navbar-toggler-icon"></span>
        </button>

        <div class="collapse navbar-collapse" id="navbarText" style="text-align:left;">
            <ul class="navbar-nav mr-auto">
                <li class="nav-item <?php if($page == 'dashboard.php'){ echo 'active';}?>">
                    <a class="nav-link" href="dashboard.php">Home<?php if($page == 'dashboard.php'){ echo ' <span class="sr-only">(current)</span>';}?></a>
                </li>
                <li class="nav-item <?php if($page == 'manage-news.php'){ echo 'active';}?>">
                    <a class="nav-link" href="manage-news.php">News<?php if($page == 'manage-news.php'){ echo ' <span class="sr-only">(current)</span>';}?></a>
                </li>
                <li class="nav-item <?php if($page == 'manage-guestbook.php'){ echo 'active';}?>">
                    <a class="nav-link" href="manage-guestbook.php">Guestbook<?php if($page == 'manage-guestbook.php'){ echo ' <span class="sr-only">(current)</span>';}?></a>
                </li>
                <li class="nav-item <?php if($page == 'manage-config.php'){ echo 'active';}?>">
                    <a class="nav-link" href="#">Settings<?php if($page == 'manage-config.php'){ echo ' <span class="sr-only">(current)</span>';}?></a>
                </li>
            </ul>    
            <span class="navbar-text" style="float:right;">
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Logged in as <?php echo htmlspecialchars($_SESSION["username"]); ?> - <a href="logout.php">Logout</a>
            </span>
        </div>
    </nav>
</div>