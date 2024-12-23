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
    $fontBold = './fonts/nimbus-sans-l/NimbusSanL-Bol.otf';
    $fontRegular = './fonts/nimbus-sans-l/NimbusSanL-Reg.otf';
    $fontItalic = './fonts/nimbus-sans-l/NimbusSanL-RegIta.otf';
    if (!(trim($data->zug1->abw).trim($data->zug1->vonnach).trim($data->zug1->nr).trim($data->zug1->via)))
    {
        $mode = "infoOnly";
    }
    $bg = imagecreatetruecolor(240, 240);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    
    // Stunden und Minute
    if ($mode != "infoOnly")
    {
        $rawTime = $data->zug1->zeit;
        if (!$rawTime) $rawTime = "";
        if (str_contains($rawTime, ":"))
        {
            $time = explode(":",$rawTime);
            addResizedTextToImage($time[0].":".$time[1],12,$fontBold,"#ffffff",1,1,$bg,3,15);
        } else {
            if (strlen($rawTime))
            {
                addResizedTextToImage($rawTime,12,$fontBold,"#ffffff",1,1,$bg,3,15);
            }
        }
    }
    
    // Hinweis
    $vonnNachY = 99;
    $hinweis = trim($data->zug1->hinweis);
    $abw = trim($data->zug1->abw);
    if ($hinweis)
    {
        // wenn  Hinweis mit andere infos
        if ($mode != "infoOnly")
        {
            $orange = imagecolorallocate($bg, 255, 0, 0);
            imagefilledrectangle($bg, 3, 78, 119, 107, $orange);
            $show = $hinweis;
            
            $text = wrapText($show,$fontRegular,8.25,111);
    
            addResizedTextToImage($text,8.25,$fontRegular,"#ffffff",1,1,$bg,5,80,$align="top-left",true,0.9);
            addResizedTextToImage($text,8.25,$fontRegular,"#ffffff",1,1,$bg,5,80,$align="top-left",true,0.9);
            $vonnNachY = 75;
        } else { 
            // ohne andere Infos, nur einen Hinweis anzeigen
            $fs = 12.75;
            $text = wrapText($hinweis,$fontBold,$fs,156);
    
            addResizedTextToImage($text,$fs,$fontRegular,"#fff048",1,1,$bg,5,45,$align="top-left",true,0.9);
            addResizedTextToImage($text,$fs,$fontRegular,"#fff048",1,1,$bg,4,45,$align="top-left",true,0.9);
        }
        
    } 

    // Von Nach
    $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
    
    // Wordwrap
    $text = wrapText($vonnach,$fontRegular,11.85,111);

    addResizedTextToImage($text,11.85,$fontRegular,"#ffffff",1,1,$bg,5,$vonnNachY,$align="bottom-left",true,0.9);
    addResizedTextToImage($text,11.85,$fontRegular,"#ffffff",1,1,$bg,5,$vonnNachY,$align="bottom-left",true,0.9);
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
        imagecopy($bg,$fg,129,13,0,0,imagesx($fg),imagesy($fg));
    
        $dot = imagecreatefrompng("./img/dot.png");
        //imagecopy($bg,$dot,84,15,0,0,imagesx($dot),imagesy($dot));
    
        $vias = explode("-",$data->zug1->via);
        $xpos = 141;
        $ypos = 21;
        for ($i=0; $i < count($vias); $i++) { 
            $via = trim($vias[$i]);
            
            addResizedTextToImage($via,7.5,$fontRegular,"#ffffff",1,1,$bg,$xpos,$ypos);
            addResizedTextToImage($via,7.5,$fontRegular,"#ffffff",1,1,$bg,$xpos,$ypos);
            imagecopy($bg,$dot,126,15-14+$ypos,0,0,imagesx($dot),imagesy($dot));
            $ypos += 16;
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
            imagefilledrectangle($bg, 65, 17, 117, 30, $blau);
            addResizedTextToImage("+".$abw."’",12,$fontBold,"#fff048",1,1,$bg,68,15);
            $showTrain = false;
        } else {
            addResizedTextToImage("+".$abw."’",12,$fontBold,"#fff048",1,1,$bg,5,36);
        }
    }

    // Zug nur, wenn keine Verspätung
    if ($showTrain)
    {

        /*
        ICE TGV PE BEX GEX EC IC IR NJ RJX VAE EXT CNL = rot
        R auf Weiß wie S
        PE GEX PE BEX PE kursiv, GEX normal
        */

        $onRed = ["ICE", "TGV", "PE", "BEX", "GEX", "EC", "IC", "IR", "NJ", "RJX", "VAE", "EXT", "CNL"];
        $nr = $data->zug1->nr;
        $type = "";
        $t = "";
        preg_match('/([a-zA-Z]+)(\d*)/', $nr, $matches, PREG_OFFSET_CAPTURE);
        $t = isset($matches[1][0]) ? $matches[1][0] : "";

        $fs = 7.8;
        $y = 18;
        $zahl = preg_replace("/[^0-9]/", '', $nr); 
        if (str_starts_with(strtolower($nr),"ic") 
        && !str_starts_with(strtolower($nr),"ice")
        && !str_starts_with(strtolower($nr),"icn"))
        {
            $fg = imagecreatefrompng("./img/ic.png");
            imagecopy($bg,$fg,71,17,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y);
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y);
        }
        elseif (str_starts_with(strtolower($nr),"ec"))
        {
            $fg = imagecreatefrompng("./img/ec.png");
            imagecopy($bg,$fg,65,17,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y);
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y);
        } 
        elseif (str_starts_with(strtolower($nr),"icn"))
        {
            $fg = imagecreatefrompng("./img/icn.png");
            imagecopy($bg,$fg,71,17,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y);
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y);
        } 
        elseif (str_starts_with(strtolower($nr),"ir"))
        {
            $fg = imagecreatefrompng("./img/ir.png");
            imagecopy($bg,$fg,71,17,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y+1);
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,96,$y+1);
        } 
        elseif (str_starts_with(strtolower($nr),"vae"))
        {
            $fg = imagecreatefrompng("./img/vae.png");
            imagecopy($bg,$fg,71,17,0,0,imagesx($fg),imagesy($fg));
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,100,$y);
            addResizedTextToImage($zahl,$fs,$fontRegular,"#ffffff",1,1,$bg,100,$y);
        } 
        elseif (str_starts_with(strtolower($nr),"re"))
        {
            $white = imagecolorallocate($bg, 255, 255, 255);
            imagefilledrectangle($bg, 72, 17, 117, 30, $white);
            addResizedTextToImage("RE".$zahl,7.95,$fontRegular,"#ff0000",1,1,$bg,74,$y);
            addResizedTextToImage("RE".$zahl,7.95,$fontRegular,"#ff0000",1,1,$bg,74,$y);
        }
        elseif (strtolower($t) == "s" || strtolower($t) == "r")
        {
            $white = imagecolorallocate($bg, 255, 255, 255);
            
            
            imagefilledrectangle($bg, 72, 17, 119, 30, $white);
            addResizedTextToImage($t.$zahl,7.95,$fontRegular,"#000000",1,1,$bg,74,$y);
            addResizedTextToImage($t.$zahl,7.95,$fontRegular,"#000000",1,1,$bg,74,$y);
        }
        elseif (strtolower($nr) == "pe bex" || strtolower($nr) == "pe gex")
        {
            $white = imagecolorallocate($bg, 255, 0, 0);
            
            
            imagefilledrectangle($bg, 65, 17, 117, 30, $white);
            addResizedTextToImage($nr,7.95,$fontItalic,"#ffffff",1,1,$bg,66,$y);
            addResizedTextToImage($nr,7.95,$fontItalic,"#ffffff",1,1,$bg,66,$y);
        }
        elseif (in_array(strtoupper($t), $onRed))
        {
            $white = imagecolorallocate($bg, 255, 0, 0);
            
            
            imagefilledrectangle($bg, 65, 17, 117, 30, $white);
            addResizedTextToImage($t.$zahl,7.95,$fontItalic,"#ffffff",1,1,$bg,66,$y);
            addResizedTextToImage($t.$zahl,7.95,$fontItalic,"#ffffff",1,1,$bg,66,$y);
        } else {
            addResizedTextToImage($nr,8.25,$fontRegular,"#ffffff",1,1,$bg,74,17);
            addResizedTextToImage($nr,8.25,$fontRegular,"#ffffff",1,1,$bg,74,17);
        }
    }
    

    // Gleis
    //addResizedTextToImage("Gleis",12,$fontBold,"#00000",0.5,1,$bg,27,34,"center");
    
    // Gleis Nr
    //addResizedTextToImage($data->gleis,33,$fontBold,"#00000",0.5,1,$bg,27,110,"center");
    
    
    // $fg = imagecreatefrompng("./img/fg.png");
    // imagecopy($bg,$fg,0,0,0,0,imagesx($fg),imagesy($fg));
    
    // Duplizieren
    imagecopy($bg,$bg,0,120,0,0,240,120);
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