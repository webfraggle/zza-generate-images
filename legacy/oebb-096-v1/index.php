<?php
error_reporting(E_ALL);
ini_set("display_errors", 1);


include_once("gfx_functions.inc.php");
include_once("cors.inc.php");

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
    $fontRegular = './fonts/Roboto/Roboto-Light.ttf';
    
    $bg = imagecreatetruecolor(160, 160);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    
    $vonnNachX = 0;
    $vonnNachY = 27;
    $w = 0;
    
    // Stunden und Minute
    $rawTime = $data->zug1->zeit;

    if (!$rawTime) $rawTime = "";
    if (str_contains($rawTime, ":"))
    {
        $time = explode(":",$rawTime);
        $w = addResizedTextToImage($time[0].":".$time[1],6.0,$fontRegular,"#ffffff",1,1.2,$bg,$vonnNachX,29);
        addResizedTextToImage($time[0].":".$time[1],6.0,$fontRegular,"#ffffff",1,1.2,$bg,$vonnNachX,29);
        addResizedTextToImage($time[0].":".$time[1],6.0,$fontRegular,"#ffffff",1,1.2,$bg,$vonnNachX,29);
        $vonnNachX += $w+2;

        // Verspätung
        $abw = trim($data->zug1->abw);
        $timeInMinutes = $time[0]*60+$time[1];
        $newTimeInMinutes = $timeInMinutes + $abw;
        $newTime = sprintf("%02d:%02d", floor($newTimeInMinutes/60), $newTimeInMinutes%60);

        if ($abw > 0)
        {
            $w = addResizedTextToImage($newTime,6.0,$fontRegular,"#fbef52",1,1.2,$bg,$vonnNachX,29);
            addResizedTextToImage($newTime,6.0,$fontRegular,"#fbef52",1,1.2,$bg,$vonnNachX,29);
            addResizedTextToImage($newTime,6.0,$fontRegular,"#fbef52",1,1.2,$bg,$vonnNachX,29);
            addResizedTextToImage($newTime,6.0,$fontRegular,"#fbef52",1,1.2,$bg,$vonnNachX,29);
            $vonnNachX += $w+2;
        }

    } else {
        if (strlen($rawTime))
        {
            $w = addResizedTextToImage($rawTime,6.0,$fontRegular,"#ffffff",1,1.2,$bg,$vonnNachX,29);
            addResizedTextToImage($rawTime,6.0,$fontRegular,"#ffffff",1,1.2,$bg,$vonnNachX,29);
            addResizedTextToImage($rawTime,6.0,$fontRegular,"#ffffff",1,1.2,$bg,$vonnNachX,29);
            $vonnNachX += $w+2;
        }
    }
    
    
    



    

    


     // Von Nach
     $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
     addResizedTextToImage($vonnach,8.5,$fontRegular,"#ffffff",1,1,$bg,$vonnNachX,$vonnNachY);
     addResizedTextToImage($vonnach,8.5,$fontRegular,"#ffffff",1,1,$bg,$vonnNachX,$vonnNachY);
     addResizedTextToImage($vonnach,8.5,$fontRegular,"#ffffff",1,1,$bg,$vonnNachX,$vonnNachY);

     // Uhrzeit
     date_default_timezone_set('Europe/Vienna');
     $uhrzeit = date("H:i:s");
     addResizedTextToImage($uhrzeit,6.8,$fontRegular,"#ffffff",1,1,$bg,157,18, "right");
     addResizedTextToImage($uhrzeit,6.8,$fontRegular,"#ffffff",1,1,$bg,157,18, "right");
     addResizedTextToImage($uhrzeit,6.8,$fontRegular,"#ffffff",1,1,$bg,157,18, "right");
    
    
    // Vias
    $vias = explode(" - ",$data->zug1->via);
    $via = implode("~",$vias);
    addResizedTextToImage($via,5.7,$fontRegular,"#ffffff",1,1,$bg,2,46);
    addResizedTextToImage($via,5.7,$fontRegular,"#ffffff",1,1,$bg,2,46);
    addResizedTextToImage($via,5.7,$fontRegular,"#ffffff",1,1,$bg,2,46);
    addResizedTextToImage($via,5.7,$fontRegular,"#ffffff",1,1,$bg,2,46);
    // Hinweis
    
    $hinweis = trim($data->zug1->hinweis);
    
    if ($hinweis)
    {
        $textColor = "#ffffff";
        if (str_starts_with($hinweis,"*"))
        {
            $hinweis = substr($hinweis,1);
            $orange = imagecolorallocate($bg, 251, 239, 82);
            $textColor = "#000000";
            imagefilledrectangle($bg, 0, 59, 160, 75, $orange);
        }
        addResizedTextToImage($hinweis,5.5,$fontRegular,$textColor,1,1.2,$bg,2,62);
        addResizedTextToImage($hinweis,5.5,$fontRegular,$textColor,1,1.2,$bg,2,62);
        addResizedTextToImage($hinweis,5.5,$fontRegular,$textColor,1,1.2,$bg,2,62);
        addResizedTextToImage($hinweis,5.5,$fontRegular,$textColor,1,1.2,$bg,2,62);
    } 


    // Zugrtyp

    $nr = $data->zug1->nr;

    $re = '/([a-zA-Z]+)([\d]+)/m';
    $res = preg_match_all($re, $nr, $matches, PREG_SET_ORDER, 0);
    $type = $nr;
    $number = "";
    if ($res) {
        $type = $matches[0][1];
        $number = $matches[0][2];
    }


    // kein match
    $x = 37;
    if ($number == "")
    {
        addResizedTextToImage($type,7,$fontBold,"#ffffff",1,1,$bg,$x,9);
    } else {
        if (strtolower($type) == "s")
        {
            $fg = imagecreatefrompng("./img/s.png");
            imagecopy($bg,$fg,$x,8,0,0,imagesx($fg),imagesy($fg));
            $x += imagesx($fg)+1;
            addResizedTextToImage($number,8,$fontBold,"#ffffff",1,1,$bg,$x,8);
            // addResizedTextToImage($number,6,$fontRegular,"#ffffff",1,1,$bg,$x,10);
        } else {
            $w = addResizedTextToImage($type,7,$fontBold,"#ffffff",1,1,$bg,$x,9);
            $x += $w+2;
            addResizedTextToImage($number,6,$fontRegular,"#ffffff",1,1,$bg,$x,10);
            addResizedTextToImage($number,6,$fontRegular,"#ffffff",1,1,$bg,$x,10);
            addResizedTextToImage($number,6,$fontRegular,"#ffffff",1,1,$bg,$x,10);
        }
        


    }


    // Duplizieren
    imagecopy($bg,$bg,0,81,0,0,160,80);

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