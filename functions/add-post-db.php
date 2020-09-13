<?php

require_once "../config.php";

// Initialize the session
session_start();

// Check if the user is logged in, if not then redirect them to login page
if(!isset($_SESSION["loggedin"]) || $_SESSION["loggedin"] !== true){
    header("location: ../login.php");
    exit;
}

$sql = 'INSERT INTO news (title, body, status, posted_by) VALUES (?, ?, ?, ?)';

if($stmt = mysqli_prepare($link, $sql)){
   
    // Bind variables to the prepared statement as parameters
    mysqli_stmt_bind_param($stmt, "ssss", $param_title, $param_body, $param_status, $param_postedby);


    // Set parameters
    $param_title = htmlspecialchars($_POST["title"]);
    $param_body = $_POST["body"];
    $param_status = htmlspecialchars($_POST["status"]);
    $param_postedby = htmlspecialchars($_POST["postedby"]);

    // Attempt to execute the prepared statement
    mysqli_stmt_execute($stmt);

    header("Location: ../manage-news.php?error=3");
    die();

} else{
    echo "ERROR: Could not prepare query: $sql. " . mysqli_error($link);
}

 

// Close statement
mysqli_stmt_close($stmt);

// Close connection
mysqli_close($link);