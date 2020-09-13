<?php

// Initialize the session
session_start();

// Include config file
require_once "config.php";

// Check if the user is logged in, if not then redirect them to login page
if(!isset($_SESSION["loggedin"]) || $_SESSION["loggedin"] !== true){
    header("location: login.php");
    exit;
}

?>

 

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">

    <title>Dashboard - Delegao.moe</title>

    <!-- Stylesheets -->
    <link rel="stylesheet" href="css/bootstrap.min.css">
    <link rel="stylesheet" href="css/all.min.css">
    <link rel="stylesheet" href="css/custom.css">

</head>

<body>
    <div id="background"></div>
    <div id="background-cover"></div>

    <div class="container">
        <nav class="navbar navbar-expand-lg navbar-dark card mt-2 mb-5">
            <a class="navbar-brand" href="#">delegao.moe area 51</a>
            <button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarText" aria-controls="navbarText" aria-expanded="false" aria-label="Toggle navigation">
                <span class="navbar-toggler-icon"></span>
            </button>

            <div class="collapse navbar-collapse" id="navbarText" style="text-align:left;">
                <ul class="navbar-nav mr-auto">
                    <li class="nav-item active">
                        <a class="nav-link" href="#">Home <span class="sr-only">(current)</span></a>
                    </li>
                    <li class="nav-item">
                        <a class="nav-link" href="manage-news.php">News</a>
                    </li>
                    <li class="nav-item">
                        <a class="nav-link" href="#">Guestbook</a>
                    </li>
                </ul>
                <span class="navbar-text" style="float:right;">
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Logged in as <?php echo htmlspecialchars($_SESSION["username"]); ?> - <a href="logout.php">Logout</a>
                </span>
            </div>
        </nav>
    </div>

    <div class="container card">
        <p class="mt-5 mb-5 text-white text-center">Work in progress</p>
    </div>



    <!-- Optional JavaScript -->

    <!-- jQuery first, then Popper.js, then Bootstrap JS -->
    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>

</body>
</html>