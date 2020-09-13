<?php
require_once "../config.php";

// Check if name is empty
if(empty(htmlspecialchars($_POST["name"]))){
    header("Location: ../guestbook.php?error=2");
    die();
} else{
    $param_name = htmlspecialchars($_POST["name"]);
}

// Check if email is empty
if(empty(htmlspecialchars($_POST["email"]))){
    $param_email = "Not provided";
} else{
    $param_email = htmlspecialchars($_POST["email"]);
}


// Check if comment is empty
if(empty(htmlspecialchars($_POST["comment"]))){
    header("Location: ../guestbook.php?error=3");
    die();
} else{
    $param_comment = htmlspecialchars($_POST["comment"]);
}

// Check if country is empty
if(empty(htmlspecialchars($_POST["country"]))){
    header("Location: ../guestbook.php?error=4");
    die();
} else{
    $param_country = htmlspecialchars($_POST["country"]);
}



$sql = 'INSERT INTO guestbook (name, email, comment, country) VALUES (?, ?, ?, ?)';

if($stmt = mysqli_prepare($link, $sql)){

// Bind variables to the prepared statement as parameters
mysqli_stmt_bind_param($stmt, "ssss", $param_name, $param_email, $param_comment, $param_country);

// Attempt to execute the prepared statement
mysqli_stmt_execute($stmt);

header("Location: ../guestbook.php?error=1");
die();
} else{
    echo "ERROR: Could not prepare query: $sql. " . mysqli_error($link);
}
 
// Close statement
mysqli_stmt_close($stmt);
 
// Close connection
mysqli_close($link);