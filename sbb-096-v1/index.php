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
    $time = explode(":",$data->zug1->zeit);
    addResizedTextToImage($time[0].".".$time[1],8.2,$fontBold,"#ffffff",1,1,$bg,3,10);
    
    // Hinweis
    $vonnNachY = 66;
    $hinweis = trim($data->zug1->hinweis);
    if ($hinweis)
    {
        $orange = imagecolorallocate($bg, 219, 73, 14);
        imagefilledrectangle($bg, 2, 51, 79, 71, $orange);
        $text = wrapText($hinweis,$fontRegular,5.5,74);
        addResizedTextToImage($text,5.5,$fontRegular,"#ffffff",1,1,$bg,3,71,$align="bottom-left",true,0.9);
        addResizedTextToImage($text,5.5,$fontRegular,"#ffffff",1,1,$bg,3,71,$align="bottom-left",true,0.9);
        $vonnNachY = 47;
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

    
    
    // Entweder Zugtyp
    
    // $nr = $data->zug1->nr;
    // $type = "";
    // if (str_starts_with(strtolower($nr),"rb")) $type = "rb.png";
    // if (str_starts_with(strtolower($nr),"ic")) $type = "ic.png";
    // if (str_starts_with(strtolower($nr),"ice")) $type = "ice.png";
    // if (str_starts_with(strtolower($nr),"re")) $type = "re.png";
    // // TODO: Oder Verspätung
    
    // if ($type)
    // {
    //     $fg = imagecreatefrompng("./img/".$type);
    //     imagecopy($bg,$fg,55,43,0,0,imagesx($fg),imagesy($fg));
    // }
    
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