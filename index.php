<?php
// Include config file
require_once "config.php";

if (isset($_GET["id"])) {
  $sql = 'SELECT * FROM news WHERE status = 1 AND id = ?';

  $stmt = mysqli_prepare($link, $sql);
  mysqli_stmt_bind_param($stmt, 'i', $_GET["id"]);
  mysqli_stmt_execute($stmt);

  $result = mysqli_stmt_get_result($stmt);
  if(mysqli_num_rows($result) > 0){
    $newsData = mysqli_fetch_assoc($result);
  } else {
    $newsData['title'] = "No posts have been found matching your criteria.";
    $newsData['posted_by'] = "Herobrine";
    $newsData['created_at'] = "2020-03-20 00:00:00";
    $newsData['body'] = "Double check that the URL has been entered correctly.";
  }
  mysqli_stmt_close($stmt);


} else {
  // If no ID has been specified, load the latest published post
  $sql = 'SELECT * FROM news WHERE status = 1 ORDER BY ID DESC LIMIT 1 ';


  $stmt = mysqli_prepare($link, $sql);
  mysqli_stmt_execute($stmt);
  
  $result = mysqli_stmt_get_result($stmt);
  if(mysqli_num_rows($result) > 0){
    $newsData = mysqli_fetch_assoc($result);
  } else {
    $newsData['title'] = "No posts have been created yet.";
    $newsData['posted_by'] = "Herobrine";
    $newsData['created_at'] = "2020-03-20 00:00:00";
    $newsData['body'] = "Maybe it is the time to write your first post?";
  }
  mysqli_stmt_close($stmt);
}
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
          <div class="card">
            <div class="card-body">
              <?php if (isset($_GET["id"])) { echo '<a href="index" class="btn btn-primary btn-sm mb-3">< Go back</a>'; }?>
              <h4 class="card-title"><?php echo $newsData['title'];?></h4>
              <h6 class="card-subtitle mb-4 text-muted"><i class="fas fa-user"></i> <?php echo $newsData['posted_by'];?>&nbsp;&nbsp;&nbsp;<i class="fas fa-calendar-alt"></i> <?php echo $newsData['created_at'];?> <?php if(isset($newsData['last_update'])) { echo "<i>(updated on ". $newsData['last_update'] .")</i>";}?></h6>
              <p class="card-text"><?php echo $newsData['body'];?></p>
              <hr>
              <a href="news" class="card-link">Older posts</a>
            </div>
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