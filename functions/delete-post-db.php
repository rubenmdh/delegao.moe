<?php

require_once "../config.php";

// Initialize the session
session_start();

// Check if the user is logged in, if not then redirect them to login page
if(!isset($_SESSION["loggedin"]) || $_SESSION["loggedin"] !== true){
    header("location: ../login.php");
    exit;
}

$sql = 'DELETE FROM news WHERE id = ?';

if($stmt = mysqli_prepare($link, $sql)){

    // Bind variables to the prepared statement as parameters
    mysqli_stmt_bind_param($stmt, "i", $param_id);

    // Set parameters
    $param_id = $_GET["id"];

    // Attempt to execute the prepared statement
    mysqli_stmt_execute($stmt);

    header("Location: ../manage-news.php?error=2");
    die();

} else{
    echo "ERROR: Could not prepare query: $sql. " . mysqli_error($link);
}

// Close statement
mysqli_stmt_close($stmt);

// Close connection
mysqli_close($link);