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
    $mode = "";
    
    $data = json_decode($json);
    $fontBold = './fonts/LLPIXEL3.ttf';
    $fontSmall = './fonts/04B_03__.TTF';
    
    $bg = imagecreatetruecolor(160, 160);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    
    
    
    // Hinweis
    $vonnNachY = 66;
    $hinweis = trim($data->zug1->hinweis);
    $abw = trim($data->zug1->abw);
    $nr = trim($data->zug1->nr);
    // if ($hinweis)
    // {
    //     // wenn  Hinweis mit andere infos
    //     if ($mode != "infoOnly")
    //     {
    //         $orange = imagecolorallocate($bg, 255, 0, 0);
    //         imagefilledrectangle($bg, 2, 52, 79, 71, $orange);
    //         $show = $hinweis;
            
    //         $text = wrapText($show,$fontRegular,5.5,74);
    
    //         addResizedTextToImage($text,5.5,$fontRegular,"#ffffff",1,1,$bg,3,53,$align="top-left",true,0.9);
    //         addResizedTextToImage($text,5.5,$fontRegular,"#ffffff",1,1,$bg,3,53,$align="top-left",true,0.9);
    //         $vonnNachY = 50;
    //     } else { 
    //         // ohne andere Infos, nur einen Hinweis anzeigen
    //         $fs = 8.5;
    //         $text = wrapText($hinweis,$fontBold,$fs,156);
    
    //         addResizedTextToImage($text,$fs,$fontRegular,"#fff048",1,1,$bg,3,30,$align="top-left",true,0.9);
    //         addResizedTextToImage($text,$fs,$fontRegular,"#fff048",1,1,$bg,3,30,$align="top-left",true,0.9);
    //     }
        
    // } 

    // Von Nach
    $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
    
    // Wordwrap
    $text = $data->zug1->zeit ." ". $vonnach;
    // $fontColor = imagecolorallocate($bg, 255, 222, 0);
    $vonnNachY = 25;
    $vonnNachX = 2;
    addResizedTextToImage($text,20,$fontBold,"#ffde00",0.5,1,$bg,$vonnNachX,$vonnNachY,$align="top-left",true,0.9);
    // addResizedTextToImage($text,10,$fontBold,"#ffde00",0.5,1,$bg,$vonnNachX,$vonnNachY,$align="bottom-left",true,0.9);
    
    // Nr:
    $dot = "•";
    for ($i=0; $i < 10; $i++) { 
        addResizedTextToImage($dot,22,$fontBold,"#ffde00",0.5,1,$bg,$i*7,22,$align="bottom-left",true,0.9);
    }
    $nrW = addResizedTextToImage($nr,15,$fontBold,"#000000",0.5,1,$bg,2,20,$align="bottom-left",true,0.9);
    $black = imagecolorallocate($bg, 0, 0, 0);
    imagefilledrectangle($bg, $nrW+3, 0, 160, 24, $black);



    // // vias
    if (trim($data->zug1->via))
    {
    
        $vias = explode("-",$data->zug1->via);
        $xpos = $nrW+14;
        $ypos = 4;
        for ($i=0; $i < count($vias); $i++) { 
            $via = trim($vias[$i]);
            $w = addResizedTextToImage($via,7,$fontSmall,"#ffde00",1,1,$bg,$xpos,$ypos);
            // addResizedTextToImage($via,5.9,$fontRegular,"#ffffff",1,1,$bg,$xpos,$ypos);
            $xpos += $w+6;
            //if ($i >= 4) break;
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