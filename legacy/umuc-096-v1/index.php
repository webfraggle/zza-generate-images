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
    $fontBold = './fonts/Roboto/Roboto-Bold.ttf';
    $fontRegular = './fonts/Roboto/RobotoCondensed-Regular.ttf';
    
    $bg = imagecreatetruecolor(160, 160);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    

     // Hinweis = Richtung XYZ
    $vonnach = mb_convert_encoding($data->zug1->hinweis, 'ISO-8859-1', 'UTF-8');
    addResizedTextToImage($vonnach,7,$fontRegular,"#ffffff",1,1,$bg,2,96);
    addResizedTextToImage($vonnach,7,$fontRegular,"#ffffff",1,1,$bg,2,96);

    // MIn.
    addResizedTextToImage("Min.",7,$fontRegular,"#ffffff",1,1,$bg,81,96);
    addResizedTextToImage("Min.",7,$fontRegular,"#ffffff",1,1,$bg,81,96);

    // Gleis
    $gleis = $data->gleis;
    addResizedTextToImage("Gleis ".$gleis,7,$fontBold,"#ffffff",1,1,$bg,125,155,"center");

    // Uhrzeit
    date_default_timezone_set('Europe/Berlin');
    $uhrzeit = explode(":", date("g:i:s"));
    $hour = intval($uhrzeit[0]);   
    $minute = intval($uhrzeit[1]);   
    $second = intval($uhrzeit[2]);   
    // $hour = 3;
    // $minute = 47;
    $hour = $hour + ($minute/60);
    $hourangle = 360-(360/12*$hour);
    $minutesangle = 360-(360/60*$minute);
    $clockx = 106;
    $clocky = 93;


    $fg = imagecreatefrompng("./img/clock-bg.png");
    $w = imagesx($fg);
    $h = imagesy($fg);

    $centerx = $clockx + $w*0.5;
    $centery = $clocky + $w*0.5;

    imagecopy($bg,$fg,$clockx,$clocky,0,0,$w,$h);

    $fg = imagecreatefrompng("./img/clock-hour.png");
    $rotation = imagerotate($fg,$hourangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
    $w = imagesx($rotation);
    $h = imagesy($rotation);
    imagecopy($bg,$rotation,intval($centerx-$w*0.5),intval($centery-$h*0.5),0,0,$w,$h);

    $fg = imagecreatefrompng("./img/clock-minutes.png");
    $rotation = imagerotate($fg,$minutesangle, imageColorAllocateAlpha($fg, 0, 0, 0, 127));
    $w = imagesx($rotation);
    $h = imagesy($rotation);
    imagecopy($bg,$rotation,intval($centerx-$w*0.5),intval($centery-$h*0.5),0,0,$w,$h);

    
    // Züge (Zeit, vonnach + nr)
    $vias = 
    [
        $data->zug1,
        $data->zug2,
        $data->zug3
    ];
    $length = count($vias);
    if ($length > 3) $length = 3;
    $re = '/(U[1-8])/m';
    $starty = 110;
    $startAbw = timeToMinutes($vias[0]->zeit)-5;
    for ($i=0; $i < $length; $i++) {
        $y = $starty + ($i*17);
        $next = $vias[$i];
        // print_r($next);
        // print_r($matches);
        $ziel = $next->vonnach;
        $nr = $next->nr;
        addResizedTextToImage($ziel,7,$fontRegular,"#ffffff",1,1,$bg,19,$y);
        addResizedTextToImage($ziel,7,$fontRegular,"#ffffff",1,1,$bg,19,$y);
        $zeit = timeToMinutes($next->zeit)-$startAbw;
        addResizedTextToImage($zeit,7,$fontRegular,"#ffffff",1,1,$bg,97,$y+8,"right");
        addResizedTextToImage($zeit,7,$fontRegular,"#ffffff",1,1,$bg,97,$y+8,"right");
        $res = preg_match($re, $nr);
        if ($res)
        {
            $fg = imagecreatefrompng("./img/".$nr.".png");
            imagecopy($bg,$fg,1,$y-2,0,0,imagesx($fg),imagesy($fg));
        } else {
            // Nur text
            addResizedTextToImage($nr,7,$fontRegular,"#ffffff",1,1,$bg,1,$y);
            addResizedTextToImage($nr,7,$fontRegular,"#ffffff",1,1,$bg,1,$y);
        }
    }
    
    

// exit();

    // Duplizieren
    imagecopy($bg,$bg,62,15,0,95,98,62);
    imagecopy($bg,$bg,0,13,106,93,54,67);

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