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





// Get the post details
$sql = "SELECT * FROM news WHERE id = ?";
$stmt = mysqli_prepare($link, $sql);

mysqli_stmt_bind_param($stmt, 'i', $_GET['id']);
mysqli_stmt_execute($stmt);

$result = mysqli_stmt_get_result($stmt);
$getData = mysqli_fetch_assoc($result);

// Close connection
mysqli_close($link);
?>

 

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">

    <title>Modify an existing post - Delegao.moe</title>

    <!-- Stylesheets -->
    <link rel="stylesheet" href="css/bootstrap.min.css">
    <link rel="stylesheet" href="css/all.min.css">
    <link rel="stylesheet" href="css/custom.css">
</head>

<body>
    <div id="background"></div>
    <div id="background-cover"></div>

    <?php include_once('header.php'); ?>

    <div class="container card shadow text-white">
      <h1 class="h3 mt-2">Modify an existing post</h1>

      <div class="container">
        <form action="functions/modify-post-db.php" method="post">
        <p>
            <label for="inputName">Title:</label>
            <input type="text" class="form-control" name="title" id="inputTitle" value="<?php echo $getData['title']; ?>">
        </p>

        <p>
            <label for="inputBody">Body:</label>
            <textarea name="body" class="form-control" id="inputBody" rows="10" cols="30"><?php echo $getData['body']; ?></textarea>
            <small>Markdown styling is supported. <a href="https://github.com/adam-p/markdown-here/wiki/Markdown-Cheatsheet" target="_blank">Cheatsheet</a></small>
        </p>

        <p>
            <label for="inputStatus">Status:</label><br>
            <label class="radio-inline"><input type="radio" value="1" name="status" <?php if($getData['status'] == 1) { echo "checked"; }?>> Published</label><br>
            <label class="radio-inline"><input type="radio" value="0" name="status" <?php if($getData['status'] == 0) { echo "checked"; }?>> Draft</label>

            <input type="hidden" name="postedby" id="inputPostedBy" value="<?php echo htmlspecialchars($_SESSION['username']);?>">
            <input type="hidden" name="ann_id" id="inputPostedBy" value="<?php echo $_GET['id'];?>">
        </p>   

        <div class="mb-3" align="right">
          <a href="manage-news.php" class="btn btn-secondary" type="button" data-dismiss="modal">Cancel</a>
          <input class="btn btn-primary" type="submit" value="Submit">
        </div>
    </div>


    <!-- Optional JavaScript -->

    <!-- jQuery first, then Popper.js, then Bootstrap JS -->
    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>
</body>
</html>