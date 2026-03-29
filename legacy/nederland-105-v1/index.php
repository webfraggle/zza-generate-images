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

    $bg = imagecreatetruecolor(240, 240);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);


    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));

    $fontColor = "#0415a1";

    // Gleis
    $gleis = $data->gleis;
    addResizedTextToImage($gleis,15.75,$fontBold,"#024ddc",1,1,$bg,15,34,"center");

    // Von Nach
    $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
    addResizedTextToImage($vonnach,14.625,$fontRegular,$fontColor,1,1,$bg,47,34);
    addResizedTextToImage($vonnach,14.625,$fontRegular,$fontColor,1,1,$bg,47,34);

    // Zeit
    $text = $data->zug1->zeit . " " .mb_convert_encoding($data->zug1->nr, 'ISO-8859-1', 'UTF-8');
    addResizedTextToImage($text,11.25,$fontRegular,$fontColor,1,1,$bg,47,15);
    addResizedTextToImage($text,11.25,$fontRegular,$fontColor,1,1,$bg,47,15);

    $text = wrapText($data->zug1->via,$fontRegular,11.25,126);
    addResizedTextToImage($text,11.25,$fontRegular,$fontColor,1,1,$bg,48,56);

    if ($data->zug2->vonnach)
    {
        $text = $data->zug2->zeit . " " .$data->zug2->nr. " ".$data->zug2->vonnach;
        $darkblue = imagecolorallocate($bg, 4, 21, 161);
        imagefilledrectangle($bg, 45, 95, 173, 110, $darkblue);
        addResizedTextToImage($text,9,$fontRegular,"#ffffff",1,1,$bg,48,97);
        addResizedTextToImage($text,9,$fontRegular,"#ffffff",1,1,$bg,48,97);
    }

    imagecopy($bg,$bgimg,183,0,183,0,57,120);




// Uhrzeit
date_default_timezone_set('Europe/Berlin');
$uhrzeit = explode(":", date("g:i:s"));
$hour = intval($uhrzeit[0]);
$minute = intval($uhrzeit[1]);
$second = intval($uhrzeit[2]);
// $hour = 0;
// $minute = 15;
$hour = $hour + ($minute/60);
$hourangle = 360-(360/12*$hour);
$minutesangle = 360-(360/60*$minute);
$secondsangle = 360-(360/60*$second);
$clockx = 211;
$clocky = 33;

$fg = imagecreatefrompng("./img/clock-hour.png");
$w = imagesx($fg);
$h = imagesy($fg);
// $centerx = $clockx + $w*0.5;
// $centery = $clocky + $w*0.5;

$rotation = imagerotate($fg,$hourangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
$w = imagesx($rotation);
$h = imagesy($rotation);
imagecopy($bg,$rotation,intval($clockx-$w*0.5),intval($clocky-$h*0.5),0,0,$w,$h);

$fg = imagecreatefrompng("./img/clock-minutes.png");
$rotation = imagerotate($fg,$minutesangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
$w = imagesx($rotation);
$h = imagesy($rotation);
imagecopy($bg,$rotation,intval($clockx-$w*0.5),intval($clocky-$h*0.5),0,0,$w,$h);

$fg = imagecreatefrompng("./img/clock-sec.png");
$rotation = imagerotate($fg,$secondsangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
$w = imagesx($rotation);
$h = imagesy($rotation);
imagecopy($bg,$rotation,intval($clockx-$w*0.5),intval($clocky-$h*0.5),0,0,$w,$h);


// exit();

    // Duplizieren
    imagecopy($bg,$bg,205,120,0,0,35,120);
    imagecopy($bg,$bg,0,120,183,0,57,120);
    imagecopy($bg,$bg,59,120,37,0,144,120);

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