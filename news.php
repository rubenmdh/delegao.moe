<?php
// Include config file
require_once "config.php";

$sql = 'SELECT * FROM news WHERE status = 1 ORDER BY ID DESC';

if($result = mysqli_query($link, $sql)){
  function newsData($result){
    if(mysqli_num_rows($result) > 0){
      
      while($row = mysqli_fetch_array($result)){
        echo '<div class="card mb-5">';
          echo '<div class="card-body">';
            echo '<h4 class="card-title">'. $row["title"] .'</h4>';
              echo '<h6 class="card-subtitle mb-4 text-muted"><i class="fas fa-user"></i> '. $row["posted_by"] .'&nbsp;&nbsp;&nbsp;<i class="fas fa-calendar-alt"></i> '. $row["created_at"]; 
              if(isset($row["last_update"])) { echo ' <i>(updated on '. $row["last_update"] .')</i>';}
              echo '</h6>';
              echo '<p class="card-text">'. mb_strimwidth($row["body"], 0, 250, "..." ).'</p>';
              echo '<a class="btn btn-outline-primary btn-sm" href="index.php?id='. $row["id"] .'" role="button">Read more</a>';
              echo '</div>';
          echo '</div>';
      }
      mysqli_free_result($result);
    } else{
      echo '<div class="card mb-5">';
        echo '<div class="card-body">';
          echo '<h4 class="card-title">No posts have been created yet.</h4><p>Maybe it is the time to write your first post?</p>';
        echo '</div>';
      echo '</div>';
    }
  }
}
 
/*
$result = mysqli_stmt_get_result($stmt);
if(mysqli_num_rows($result) > 0){
  $newsData = mysqli_fetch_assoc($result);
} else {
  $newsData['title'] = "No posts have been created yet.";
  $newsData['posted_by'] = "Herobrine";
  $newsData['created_at'] = "2020-03-20 00:00:00";
  $newsData['body'] = "Maybe it is the time to write your first post?";
}
mysqli_stmt_close($stmt);*/
mysqli_close($link);
?>

<!doctype html>
<html lang="en">   
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">

    <!-- Stylesheets -->
    <link rel="stylesheet" href="css/bootstrap.min.css">
    <link rel="stylesheet" href="css/all.min.css">
    <link rel="stylesheet" href="css/custom.css">

    <title>Delegao.moe</title>

  </head>
  <body>
    <div id="background"></div>
    <div id="background-cover"></div>
    <div class="container text-white" style="margin-top: 6.5rem; height: auto;">
      <div class="row" style="height: auto;">
        <div id="memo" class="col-xs-12 col-lg-4 order-lg-2 mb-5">
            <?php include_once ('sidebar.php');?>
        </div>
        <div id="blog" class="col-xs-12 col-lg-8 order-lg-1" style="margin-bottom: 6.5rem;">
          <?php
            newsData($result);
          ?>
        </div>
      </div>

    <!-- Optional JavaScript -->
    <!-- jQuery first, then Popper.js, then Bootstrap JS -->
    <script src="js/jquery-3.4.1.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="js/bootstrap.min.js"></script>
  </body>
</html>