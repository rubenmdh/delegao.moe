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

$sql = 'SELECT * FROM news ORDER BY id DESC';
  if($result = mysqli_query($link, $sql)){
    function getNews($result){
      if(mysqli_num_rows($result) > 0){     
        while($row = mysqli_fetch_array($result)){
          echo'<tr>';
            echo'<td>' . $row["id"] . '</td>';
            echo'<td>' . $row["title"] . '</td>';
            if($row['status'] == 1) { echo "<td>Published</td>"; } else { echo "<td>Draft</td>"; }
            echo'<td>' . $row["tags"] . '</td>';
            echo'<td>' . $row["posted_by"] . '</td>';
            echo'<td>' . $row["created_at"] . '</td>';
            if (is_null($row["last_update"])){
                echo '<td>Never updated</td>';
            } else {
                echo'<td>' . $row["last_update"] . '</td>';
            }
            echo '<td><a href="modify-post.php?id=' . $row["id"] . '" class="btn btn-secondary" role="button">Modify</a>&nbsp;<a href="functions/delete-post-db.php?id=' . $row["id"] . '" class="btn btn-danger" role="button" onclick="return checkDelete()"><i class="fas fa-trash"></i></a>';
          echo'</tr>';
        }
        mysqli_free_result($result);
      } else{
        echo "<tr><td>You haven't created any announcement yet.</tr>";
      }
    }
  }
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">

    <title>Manage news - Delegao.moe</title>

    <!-- Stylesheets -->
    <link rel="stylesheet" href="css/bootstrap.min.css">
    <link rel="stylesheet" href="css/all.min.css">
    <link rel="stylesheet" href="css/custom.css">

    <style>
        .table{
            color: white;
        }
    </style>



    <script language="JavaScript" type="text/javascript">
        function checkDelete(){
            return confirm('Are you sure?');
        }
    </script>



</head>

<body>
    <div id="background"></div>
    <div id="background-cover"></div>

    <?php include_once('header.php'); ?>

    <div class="container card shadow text-white">
      <h1 class="h3 mt-2">News management<a class="btn btn-primary" type="button" style="float: right;" href="new-post.php">New post</a></h1>
        <div class="card-body">
          <div class="table-responsive">
            <table class="table table-bordered" id="dataTable" width="100%" cellspacing="0">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Title</th>
                  <th>Status</th>
                  <th>Tags</th>
                  <th>Posted by</th>
                  <th>Date</th>
                  <th>Last update</th>
                  <th></th>
                </tr>
              </thead>
              <tfoot>
                <tr>
                  <th>ID</th>
                  <th>Title</th>
                  <th>Status</th>
                  <th>Tags</th>
                  <th>Posted by</th>
                  <th>Date</th>
                  <th>Last update</th>
                  <th></th>
                </tr>
              </tfoot>
              <tbody>
                <?php
                  getNews($result);
                ?>
              </tbody>

            </table>
          </div>
        </div>
    </div>



    <!-- Optional JavaScript -->

    <!-- jQuery first, then Popper.js, then Bootstrap JS -->
    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>

</body>
</html>