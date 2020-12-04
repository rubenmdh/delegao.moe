<?php
// Include config file
require_once "config.php";

$sql = 'SELECT * FROM guestbook ORDER BY id DESC';
  if($result = mysqli_query($link, $sql)){
    function guestData($result){
      if(mysqli_num_rows($result) > 0){
        
        while($row = mysqli_fetch_array($result)){
          echo'<tr>';
            echo'<td>' . htmlspecialchars($row["name"]) . '</td>';
            echo'<td>' . str_replace("@", "[at)", htmlspecialchars($row["email"])) . '</td>';
            echo'<td>' . htmlspecialchars($row["comment"]) . '</td>';
            echo'<td>' . htmlspecialchars($row["country"]) . '</td>';
            echo'<td>' . htmlspecialchars($row["date"]) . '</td>';
        }
        mysqli_free_result($result);
      } else{
        echo "<tr><td>This could be you.</tr>";
      }
    }
  }


  // Error handling messages
  if (isset($_GET['error'])) {  
    switch($_GET['error']){
      case "1":
        $error_message = "<strong>Yay!</strong> You have been added to the guestbook. <font size='2.25em'><i>Your freebie might be on its way.</i></font>";
        break;
      case "2":
        $error_message = "Please enter your name. It is not that difficult.";
        break;
      case "3":
        $error_message = "Don't be lazy and write some words in the comments section.";
        break;
      case "4":
        $error_message = "You forgot to add your country. If this was intentional, please write Anonymous Country.";
        break;
      case "5":
        $error_message = "Please enter an answer. It can be found on the <a href='about-me.php'>About me</a> page.";
        break;
      case "6":
        $error_message = "I don't feel like having that today. Check your answer. CaSe SeNsItIvE";
        break;
      default:
      $error_message = "Are you sure there is an error? You might want to double check what you are trying to do...";
        break;
    }
  }
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

    <title>Guestbook - Delegao.moe</title>

    <style>
        .table{
            color: white;
        }
    </style>

  </head>
  <body>
    <div id="background"></div>
    <div id="background-cover"></div>
    <div class="container text-white" style="margin-top: 6.5rem; margin-bottom: 6.5rem; height: auto;">
        <div id="guestbook">
          <div class="card">
            <div class="card-body">
              <h4 class="card-title">Guestbook <a class="btn btn-primary" type="button" style="float: right;" data-toggle="modal" data-target="#addGuestModal"><i class="fas fa-user-plus"></i> Add me</a></h4>
              <h6 class="card-subtitle mb-4 text-muted">Get a freebie from your visit</h6>
              <?php
              if (isset($_GET['error'])) { 
                echo '<div class="alert alert-dark container" role="alert">';
                echo $error_message;
                echo '</div>';
              }
              ?>
                <div class="card-body">
                  <div class="table-responsive">
                    <table class="table table-bordered" id="dataTable" width="100%" cellspacing="0">
                      <thead>
                        <tr>
                          <th>Name</th>
                          <th>Email</th>
                          <th>Comment</th>
                          <th>Country</th>
                          <th>Date</th>
                        </tr>
                      </thead>
                      <tfoot>
                        <tr>
                          <th>Name</th>
                          <th>Email</th>
                          <th>Comment</th>
                          <th>Country</th>
                          <th>Date</th>
                        </tr>
                      </tfoot>
                      <tbody>
                        <?php
                          guestData($result);
                        ?>
                      </tbody>
                    </table>
                  </div>
                </div>
            </div>
          </div>
        </div>
      


      <div id="minimenu" class="container card text-white">
        <div id="minimenucontent" class="ml-3 mt-3 mb-2 col">
          <a href="index" class="h4 font-weight-light">< go back</a>
          <hr>
          <p class="display-4"><b>delegao</b>.moe</p>
        </div>
      </div>
    </div>

<!-- Modal -->
<div class="modal fade" id="addGuestModal" tabindex="-1" role="dialog" aria-labelledby="addGuestModalLabel" aria-hidden="true">
  <div class="modal-dialog" role="document">
    <div class="modal-content">
      <div class="modal-header">
        <h5 class="modal-title" id="addGuestModalLabel">Add yourself to the guestbook</h5>
        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
          <span aria-hidden="true">&times;</span>
        </button>
      </div>
      <div class="modal-body">
        <form action="functions/add-guest-db.php" method="post">
          <div class="alert alert-secondary alert-dismissible fade show" role="alert">
            <i class="fas fa-info-circle"></i> <b>Tip of the day:</b> Don't be a dick.
            <button type="button" class="close" data-dismiss="alert" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <p>
            <label for="inputName">Name:</label>
            <input type="text" class="form-control" name="name" id="inputName" required>
          </p>
          <p>
            <label for="inputEmail">Email:</label>
            <input type="email" class="form-control" name="email" id="inputEmail">
          </p>
          <p>
            <label for="inputComment">Comment:</label>
            <textarea name="comment" class="form-control" id="inputComment" rows="5" cols="20"></textarea>
          </p>
          <p>
            <label for="inputCountry">Country:</label>
            <input type="country" class="form-control" name="country" id="inputCountry">
            <input type="text" name="password" style="display:none !important" tabindex="-1" autocomplete="off">
          </p>
          <p>
            <label for="inputDish">Dish of the day:</label>
            <input type="text" class="form-control" placeholder='It can be found on the "About me" page.' name="dishoftheday" id="inputDish">
              <br>
            <input type="checkbox" id="inputCheckbox" value="Bird"> <label for="inputCheckbox">Are you a bird?</label>
          </p>
         
      </div>
      <div class="modal-footer">
        <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
        <input class="btn btn-primary" type="submit" value="Submit">
        </form>
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