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
    $fontBold = './fonts/nimbus-sans-l/NimbusSanL-Bol.otf';
    $fontRegular = './fonts/nimbus-sans-l/NimbusSanL-Reg.otf';
    
    $bg = imagecreatetruecolor(160, 160);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    
    // Stunden und Minute
    $rawTime = $data->zug1->zeit;
    if (!$rawTime) $rawTime = "";
    if (str_contains($rawTime, ":"))
    {
        $time = explode(":",$rawTime);
        addResizedTextToImage($time[0].":".$time[1],8.2,$fontBold,"#ffffff",1,1,$bg,3,10);
    } else {
        if (strlen($rawTime))
        {
            addResizedTextToImage($rawTime,8.2,$fontBold,"#ffffff",1,1,$bg,3,10);

        }
    }
    
    // Hinweis
    $vonnNachY = 66;
    $hinweis = trim($data->zug1->hinweis);
    $abw = trim($data->zug1->abw);
    if ($hinweis)
    {
        // wenn  Hinweis mit andere infos
        if (trim($data->zug1->abw).trim($data->zug1->vonnach).trim($data->zug1->nr).trim($data->zug1->zeit).trim($data->zug1->via))
        {
            $orange = imagecolorallocate($bg, 255, 0, 0);
            imagefilledrectangle($bg, 2, 52, 79, 71, $orange);
            $show = $hinweis;
            
            $text = wrapText($show,$fontRegular,5.5,74);
    
            addResizedTextToImage($text,5.5,$fontRegular,"#ffffff",1,1,$bg,3,53,$align="top-left",true,0.9);
            addResizedTextToImage($text,5.5,$fontRegular,"#ffffff",1,1,$bg,3,53,$align="top-left",true,0.9);
            $vonnNachY = 50;
        } else { 
            // ohne andere Infos, nur einen Hinweis anzeigen
            $fs = 8.5;
            $text = wrapText($hinweis,$fontBold,$fs,156);
    
            addResizedTextToImage($text,$fs,$fontRegular,"#fff048",1,1,$bg,3,30,$align="top-left",true,0.9);
            addResizedTextToImage($text,$fs,$fontRegular,"#fff048",1,1,$bg,3,30,$align="top-left",true,0.9);
        }
        
    } 

    // Von Nach
    $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
    
    // Wordwrap
    $text = wrapText($vonnach,$fontRegular,7.9,74);

    addResizedTextToImage($text,7.9,$fontRegular,"#ffffff",1,1,$bg,3,$vonnNachY,$align="bottom-left",true,0.9);
    addResizedTextToImage($text,7.9,$fontRegular,"#ffffff",1,1,$bg,3,$vonnNachY,$align="bottom-left",true,0.9);
    // addMultilineResizedTextToImage($text,7.9,10,$fontRegular,"#ffffff",1,1,$bg,3,51);
    $fontColor = imagecolorallocate($bg, 255, 255, 255);
    // imagefttext($bg, 7.9*96/72, 0, 80, 20, $fontColor, $fontRegular, "Test");
    // imagefttext($bg, 7.9*96/72, 0, 80, 20, $fontColor, $fontRegular, "Test");

    // addResizedTextToImage("Test",7.9,$fontRegular,"#ffffff",1,1,$bg,80,30,$align="top-left",true,0.8);
    // addResizedTextToImage("Test",7.9,$fontRegular,"#ffffff",1,1,$bg,80,30,$align="top-left",true,0.8);
        
    
    // vias
    // Dots
    if (trim($data->zug1->via))
    {
        $fg = imagecreatefrompng("./img/dottet-line.png");
        imagecopy($bg,$fg,86,8,0,0,imagesx($fg),imagesy($fg));
    
        $dot = imagecreatefrompng("./img/dot.png");
        imagecopy($bg,$dot,84,15,0,0,imagesx($dot),imagesy($dot));
    
        $vias = explode("-",$data->zug1->via);
        $xpos = 94;
        $ypos = 14;
        for ($i=0; $i < count($vias); $i++) { 
            $via = trim($vias[$i]);
            
            addResizedTextToImage($via,5.9,$fontRegular,"#ffffff",1,1,$bg,$xpos,$ypos);
            addResizedTextToImage($via,5.9,$fontRegular,"#ffffff",1,1,$bg,$xpos,$ypos);
            imagecopy($bg,$dot,84,15-14+$ypos,0,0,imagesx($dot),imagesy($dot));
            $ypos += 11;
            if ($i >= 4) break;
        }
    }

    
    $showTrain = true;
    $abw = intval($abw);
    if ($abw > 0)
    {
        if ($hinweis)
        {
            $blau = imagecolorallocate($bg, 19, 42, 155);
            imagefilledrectangle($bg, 43, 11, 48+30, 11+9, $blau);
            addResizedTextToImage("+".$abw."’",8.2,$fontBold,"#fff048",1,1,$bg,45,10);
            $showTrain = false;
        } else {
            addResizedTextToImage("+".$abw."’",8.2,$fontBold,"#fff048",1,1,$bg,3,24);
        }
    }

    // Zug nur, wenn keine Verspätung
    if ($showTrain)
    {

        $nr = $data->zug1->nr;
        $type = "";
        $zahl = preg_replace("/[^0-9]/", '', $nr); 
        if (str_starts_with(strtolower($nr),"ic") 
        && !str_starts_with(strtolower($nr),"ice")
        && !str_starts_with(strtolower($nr),"icn"))
        {
            $fg = imagecreatefrompng("./img/ic.png");
            imagecopy($bg,$fg,48,11,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,64,12);
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,64,12);
        }
        elseif (str_starts_with(strtolower($nr),"ec"))
        {
            $fg = imagecreatefrompng("./img/ec.png");
            imagecopy($bg,$fg,43,11,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,65,12);
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,65,12);
        } 
        elseif (str_starts_with(strtolower($nr),"icn"))
        {
            $fg = imagecreatefrompng("./img/icn.png");
            imagecopy($bg,$fg,48,11,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,64,12);
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,64,12);
        } 
        elseif (str_starts_with(strtolower($nr),"ir"))
        {
            $fg = imagecreatefrompng("./img/ir.png");
            imagecopy($bg,$fg,48,11,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,64,12);
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,64,12);
        } 
        elseif (str_starts_with(strtolower($nr),"vae"))
        {
            $fg = imagecreatefrompng("./img/vae.png");
            imagecopy($bg,$fg,48,11,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,67,12);
            addResizedTextToImage($zahl,5.2,$fontRegular,"#ffffff",1,1,$bg,67,12);
        } 
        elseif (str_starts_with(strtolower($nr),"re"))
        {
            $white = imagecolorallocate($bg, 255, 255, 255);
            imagefilledrectangle($bg, 48, 11, 48+31, 11+9, $white);
            addResizedTextToImage("RE".$zahl,5.3,$fontRegular,"#ff0000",1,1,$bg,49,12);
            addResizedTextToImage("RE".$zahl,5.3,$fontRegular,"#ff0000",1,1,$bg,49,12);
        }
        elseif (str_starts_with(strtolower($nr),"s"))
        {
            $white = imagecolorallocate($bg, 255, 255, 255);
            imagefilledrectangle($bg, 48, 11, 48+31, 11+9, $white);
            addResizedTextToImage("S".$zahl,5.3,$fontRegular,"#000000",1,1,$bg,49,12);
            addResizedTextToImage("S".$zahl,5.3,$fontRegular,"#000000",1,1,$bg,49,12);
        } else {
            addResizedTextToImage($nr,5.5,$fontRegular,"#ffffff",1,1,$bg,49,11);
            addResizedTextToImage($nr,5.5,$fontRegular,"#ffffff",1,1,$bg,49,11);
        }
    }
    

    // Gleis
    //addResizedTextToImage("Gleis",12,$fontBold,"#00000",0.5,1,$bg,27,34,"center");
    
    // Gleis Nr
    //addResizedTextToImage($data->gleis,33,$fontBold,"#00000",0.5,1,$bg,27,110,"center");
    
    
    // $fg = imagecreatefrompng("./img/fg.png");
    // imagecopy($bg,$fg,0,0,0,0,imagesx($fg),imagesy($fg));
    
    // Duplizieren
    imagecopy($bg,$bg,0,81,0,0,160,80);
    //imagecopy($bg,$bg,240-49-5,135+10,3,10,49,120);
    //imagecopy($bg,$bg,3,135+10,52,10,183,120);


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