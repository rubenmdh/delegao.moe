<?php
/*  
    Delegao.moe installer
    This installer will create the first housekeeping user and will populate the database with necessary tables

    Please note you have to enter your database details on the config.php file beforehand!
    It is absolutely mandatory to delete this file after a successful installation.
*/

// Include config file
require_once "config.php";

// Files to delete after a successful installation
$files = [
    './install.php',
    './install_step1.php',
    './install_step2.php',
    './SQL_UPLOAD.sql'
];

// Define variables and initialize with empty values
$username = $password = $confirm_password = "";
$username_err = $password_err = $confirm_password_err = "";
 
// Processing form data when form is submitted
if($_SERVER["REQUEST_METHOD"] == "POST"){
 
    // Validate username
    if(empty(trim($_POST["username"]))){
        $username_err = "Please enter a username.";
    } else{
        $username = trim($_POST["username"]);
    }

    // Validate password
    if(empty(trim($_POST["password"]))){
        $password_err = "Please enter a password.";     
    } elseif(strlen(trim($_POST["password"])) < 6){
        $password_err = "Password must have atleast 6 characters.";
    } else{
        $password = trim($_POST["password"]);
    }
    
    // Validate confirm password
    if(empty(trim($_POST["confirm_password"]))){
        $confirm_password_err = "Please confirm password.";     
    } else{
        $confirm_password = trim($_POST["confirm_password"]);
        if(empty($password_err) && ($password != $confirm_password)){
            $confirm_password_err = "Password did not match.";
        }
    }
    
    // Check input errors before inserting in database
    if(empty($username_err) && empty($password_err) && empty($confirm_password_err)){
    
        // Prepare an insert statement
        $sql = "INSERT INTO users (username, password) VALUES (?, ?)";
         
        if($stmt = mysqli_prepare($link, $sql)){
            // Bind variables to the prepared statement as parameters
            mysqli_stmt_bind_param($stmt, "ss", $param_username, $param_password);
            
            // Set parameters
            $param_username = $username;
            $param_password = password_hash($password, PASSWORD_DEFAULT); // Creates a password hash
            
            // Attempt to execute the prepared statement
            if(mysqli_stmt_execute($stmt)){
                // Delete install files after successful installation
                foreach ($files as $file) {
                    if (file_exists($file)) {
                        unlink($file);
                    } else {
                        echo "File not found";
                    }
                }

                // Inform the user and redirect to login page
                echo '<script type="text/javascript">';
                echo 'alert("Installation successful. You will now be redirected to the login page.");';
                echo 'location="login.php";';
                echo '</script>';
            } else{
                echo "Something went wrong. Please try again later.";
            }

            // Close statement
            mysqli_stmt_close($stmt);
        }
    }
    
    // Close connection
    mysqli_close($link);
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
</head>

<body>
    <div id="background"></div>
    <div id="background-cover"></div>

    <div class="container card mt-5" style="max-width: 600px;">
        <h4 class="mt-3 mb-2 text-white text-center">delegao.moe installation wizard</h4>
        
        <div class="card text-white pl-3 pt-3">
            <h5>Step 2 - Admin user creation</h5>
            <p>You are required to have an administrative user in order to post new threads and manage the guestbook entries.</p>
            <p>Enter your desired username and password. Passwords must be at least six characters long.</p>
        </div>
        <div class="container mt-3">
            <form action="<?php echo htmlspecialchars($_SERVER["PHP_SELF"]); ?>" method="post">

            <div class="form-group <?php echo (!empty($username_err)) ? 'has-error' : ''; ?>">
                <label class="sr-only" for="inlineFormInputGroup">Username</label>
                <div class="input-group mb-2">
                    <div class="input-group-prepend">
                        <div class="input-group-text"><i class="fas fa-user"></i></div>
                    </div>
                    <input type="text" class="form-control" name="username" placeholder="Username" value="<?php echo $username; ?>">
                </div>
            </div>

            <div class="form-group <?php echo (!empty($password_err)) ? 'has-error' : ''; ?>">
                <label class="sr-only" for="inlineFormInputGroup">Password</label>
                <div class="input-group mb-2">
                    <div class="input-group-prepend">
                        <div class="input-group-text"><i class="fas fa-key"></i></div>
                    </div>
                    <input type="password" class="form-control" name="password" placeholder="Password" value="<?php echo $password; ?>">
                </div>
            </div>

            <div class="form-group <?php echo (!empty($confirm_password_err)) ? 'has-error' : ''; ?>">
                <label class="sr-only" for="inlineFormInputGroup">Confirm password</label>
                <div class="input-group mb-2">
                    <div class="input-group-prepend">
                        <div class="input-group-text"><i class="fas fa-key"></i></div>
                    </div>
                    <input type="password" class="form-control" name="confirm_password" placeholder="Confirm password" value="<?php echo $confirm_password; ?>">
                </div>
            </div>

            <p class="text-white"><span class="help-block"><?php echo $username_err; ?></span></p>
            <p class="text-white"><span class="help-block"><?php echo $password_err; ?></span></p>
            <p class="text-white"><span class="help-block"><?php echo $confirm_password_err; ?></span></p>
                  
                
                <div class="form-group">
                    <input type="submit" class="btn btn-primary" value="Install">
                </div>
            </form>
        </div>
    </div>


    <!-- Optional JavaScript -->

    <!-- jQuery first, then Popper.js, then Bootstrap JS -->

    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>

</body>
</html>