<?php
error_reporting(E_ALL);
ini_set("display_errors", 1);


include_once("gfx_functions.inc.php");
include_once("cors.inc.php");
include_once("functions.inc.php");

cors();

$json = file_get_contents('php://input');
$cache = true;
$hasCache = false;
if (!$json)
{
    $json = file_get_contents("default.json");
}

$hash = sha1($json);

if ($cache)
{
    $imagefile = "./cache/".$hash.".png";
    if (file_exists($imagefile))
    {
        $hasCache = true;
    }
}

$hasCache = false; // cache immer überschreiben


if (!$hasCache)
{
    $data = json_decode($json);
    $fontBold = './fonts/nimbus-sans-l/NimbusSanL-Bol.otf';;
    $fontRegular = './fonts/nimbus-sans-l/NimbusSanL-Reg.otf';;
    
    $bg = imagecreatetruecolor(160, 160);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));

    $fontColor = "#0415a1";
    
    


    // Gleis
    $gleis = $data->gleis;
    addResizedTextToImage($gleis,10.5,$fontBold,"#024ddc",1,1,$bg,10,22,"center");

    
    
    
    
    // Von Nach
    $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
    addResizedTextToImage($vonnach,9.75,$fontRegular,$fontColor,1,1,$bg,32,24);
    addResizedTextToImage($vonnach,9.75,$fontRegular,$fontColor,1,1,$bg,32,24);

    // Zeit
    $text = $data->zug2->zeit . " " .mb_convert_encoding($data->zug1->nr, 'ISO-8859-1', 'UTF-8');
    addResizedTextToImage($text,7.5,$fontRegular,$fontColor,1,1,$bg,32,10);
    addResizedTextToImage($text,7.5,$fontRegular,$fontColor,1,1,$bg,32,10);

    $text = wrapText($data->zug1->via,$fontRegular,7.5,84);
    addResizedTextToImage($text,7.5,$fontRegular,$fontColor,1,1,$bg,32,37);

    if ($data->zug2->vonnach)
    {
        $text = $data->zug2->zeit . " " .$data->zug2->nr. " ".$data->zug2->vonnach;
        $darkblue = imagecolorallocate($bg, 4, 21, 161);
        imagefilledrectangle($bg, 30, 63, 30+85, 63+11, $darkblue);
        addResizedTextToImage($text,6,$fontRegular,"#ffffff",1,1,$bg,32,65);
        addResizedTextToImage($text,6,$fontRegular,"#ffffff",1,1,$bg,32,65);
    }

    imagecopy($bg,$bgimg,115,0,115,0,45,80);
   
    


// Uhrzeit
date_default_timezone_set('Europe/Berlin');
$uhrzeit = explode(":", date("g:i:s"));
$hour = intval($uhrzeit[0]);   
$minute = intval($uhrzeit[1]);   
$second = intval($uhrzeit[2]);   
// $hour = 11;
// $minute = 15;
$hour = $hour + ($minute/60);
$hourangle = 360-(360/12*$hour);
$minutesangle = 360-(360/60*$minute);
$clockx = 128;
$clocky = 9;

$fg = imagecreatefrompng("./img/clock-hour.png");
$w = imagesx($fg);
$h = imagesy($fg);
$centerx = $clockx + $w*0.5;
$centery = $clocky + $w*0.5;

$rotation = imagerotate($fg,$hourangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
$w = imagesx($rotation);
$h = imagesy($rotation);
imagecopy($bg,$rotation,intval($centerx-$w*0.5),intval($centery-$h*0.5),0,0,$w,$h);

$fg = imagecreatefrompng("./img/clock-minutes.png");
$rotation = imagerotate($fg,$minutesangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
$w = imagesx($rotation);
$h = imagesy($rotation);
imagecopy($bg,$rotation,intval($centerx-$w*0.5),intval($centery-$h*0.5),0,0,$w,$h);


// exit();

    // Duplizieren
    imagecopy($bg,$bg,0,80,122,0,38,80);
    imagecopy($bg,$bg,137,80,0,0,23,80);
    imagecopy($bg,$bg,38,80,23,0,99,80);
    // imagecopy($bg,$bg,0,13,106,93,54,67);

    if ($cache)
    {   
        $directory = "./cache";
        if (!is_dir($directory)) {
            if (mkdir($directory, 0777, true)) {
                // echo "Directory created successfully.";
            } else {
                // echo "Failed to create directory.";
            }
        }
        imagepng($bg, "./cache/".$hash.".png");
    }

}

// if (!$_GET['debug']) 
// {
    header("Content-type: image/png");
    $size = filesize($imagefile);
    header("Content-Transfer-Encoding: Binary"); 
    header("Content-Length: ".$size);
// }
readfile($imagefile);
exit;
?>