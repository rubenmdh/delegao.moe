<?php
/*  
    Delegao.moe installer
    This installer will create the first housekeeping user and will populate the database with necessary tables

    Please note you have to enter your database details on the config.php file beforehand!
    It is absolutely mandatory to delete this file after a successful installation.
*/

// Include config file
require_once "config.php";

if($_SERVER["REQUEST_METHOD"] == "POST"){       
        // Import database from SQL file
        $query = '';
        $sqlScript = file('SQL_UPLOAD.sql');
        foreach ($sqlScript as $line)	{
            
            $startWith = substr(trim($line), 0 ,2);
            $endWith = substr(trim($line), -1 ,1);
            
            if (empty($line) || $startWith == '--' || $startWith == '/*' || $startWith == '//') {
                continue;
            }
                
            $query = $query . $line;
            if ($endWith == ';') {
                mysqli_query($link,$query) or die('Problem in executing the SQL query' . $query);
                $query= '';		
            }
        }

}
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

    <style>
    .alignleft {
	    float: left;
    }
    .alignright {
    	float: right;
    }
    </style>
</head>

<body>
    <div id="background"></div>
    <div id="background-cover"></div>

    <div class="container card mt-5" style="max-width: 600px;">
        <h4 class="mt-3 mb-2 text-white text-center">delegao.moe installation wizard</h4>
        
        <div class="card text-white pl-3 pt-3">
            <h5>Step 1 - Database setup</h5>
            <p>The first step will create the database structure so that you don't have to do it manually.</p>
            <p>Please make sure there is no content in your database before proceeding.</p>
        </div>
        <div class="container mt-3">
            <form method="post">
                <div class="form-group">
                    <input type="submit" name="create" id="create" class="btn btn-primary" <?php if(array_key_exists('create',$_POST)){   echo "disabled";}?> value="Create database structure">
                </div>
            </form>
            <?php
            if(array_key_exists('create',$_POST)){
                echo '<p class="alignleft text-white">Database import successful</p> <a href="install_step2.php" class="btn btn-primary alignright mb-3">Continue</a>';
             }?>
        </div>
    </div>


    <!-- Optional JavaScript -->

    <!-- jQuery first, then Popper.js, then Bootstrap JS -->

    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>

</body>
</html>