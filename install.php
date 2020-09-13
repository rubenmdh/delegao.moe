<?php
/*  
    Delegao.moe installer
    This installer will create the first housekeeping user and will populate the database with necessary tables

    Please note you have to enter your database details on the config.php file beforehand!
    It is absolutely mandatory to delete this file after a successful installation.
*/
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">

    <title>Installation wizard - Delegao.moe</title>

    <!-- Stylesheets -->
    <link rel="stylesheet" href="css/bootstrap.min.css">
    <link rel="stylesheet" href="css/all.min.css">
    <link rel="stylesheet" href="css/custom.css">
</head>

<body>
    <div id="background"></div>
    <div id="background-cover"></div>

    <div class="container card text-white mt-5" style="max-width: 600px;">
        <h4 class="mt-3 mb-3 text-white text-center">delegao.moe installation wizard</h4>
        
            <p>Welcome to the delegao.moe installation wizard!</p>
            <p>This wizard will walk you through the installation of the delegao.moe script.</p>
            <p>It will try to populate the database with the required tables and it will create your housekeeping user.</p>
            <p><b>You are required to enter your database details in the <i>config.php</i> file beforehand.</b></p>

            <p>Please click on the button below to continue the installation proccess.</p>
            <a href="install_step1.php" class="btn btn-primary mb-3">Continue</a>
        </div>
    </div>


    <!-- Optional JavaScript -->

    <!-- jQuery first, then Popper.js, then Bootstrap JS -->

    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>

</body>
</html>