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
    $cache = false;
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

$hasCache = false;
if (!$hasCache)
{
    $data = json_decode($json);
    $font = './fonts/nimbus-sans-l/NimbusSanL-Bol.otf';
    
    
    $bg = imagecreatetruecolor(240, 240);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    
    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    // Abfahrt
    addResizedTextToImage("Abfahrt",10.5,$font,"#3a3c3b",0.5,1,$bg,57,35);
    
    // Stunden und Minute
    $time = explode(":",$data->zug1->zeit);
    addResizedTextToImage($time[0],12.5,$font,"#3a3c3b",1,1,$bg,119,37, "right");
    addResizedTextToImage($time[1],8.5,$font,"#3a3c3b",1,1,$bg,133,32, "center");
    
    // Von Nach
    $xpos = 54;
    $vonnach = mb_convert_encoding(mb_strtoupper($data->zug1->vonnach), 'ISO-8859-1', 'UTF-8');
    for ($i=0; $i < 13; $i++) { 
        $char =  substr($vonnach,$i,1);
        if ($char && $char != " ")
        {
            addResizedTextToImage($char,12,$font,"#3a3c3b",0.5,1,$bg,$xpos+5,105,"center");
        }
        $xpos += 14;
    }
    
    // vias
    if ($data->zug1->via)
    {
        $vias = explode("-",$data->zug1->via);
        $xpos = 147;
        $ypos = 68;
        $maxX = $xpos+87;
        for ($i=0; $i < count($vias); $i++) { 
            $via = trim($vias[$i]);
            $width = addResizedTextToImage($via,9.5,$font,"#3a3c3b",0.5,1,$bg,$xpos,$ypos,"left",false);
            // print $via ." ". $width." ".$maxX." ".($xpos+$width);
            if ($xpos+$width < $maxX)
            {
                $paint = true;
            } else {
                    $paint = false;
            }
            if ($paint) addResizedTextToImage($via,9.5,$font,"#3a3c3b",0.5,1,$bg,$xpos,$ypos);
            $xpos += $width+2;
        }
    }
    
    
    // Entweder Zugtyp
    
    $nr = $data->zug1->nr;
    $type = "";
    if (str_starts_with(strtolower($nr),"rb")) $type = "rb.png";
    if (str_starts_with(strtolower($nr),"ic")) $type = "ic.png";
    if (str_starts_with(strtolower($nr),"ice")) $type = "ice.png";
    if (str_starts_with(strtolower($nr),"re")) $type = "re.png";
    // TODO: Oder Verspätung
    
    if ($type)
    {
        $fg = imagecreatefrompng("./img/".$type);
        imagecopy($bg,$fg,55,53,0,0,imagesx($fg),imagesy($fg));
    }
    
    // Gleis
    addResizedTextToImage("Gleis",12,$font,"#00000",0.5,1,$bg,27,34,"center");
    
    // Gleis Nr
    addResizedTextToImage($data->gleis,33,$font,"#00000",0.5,1,$bg,27,90,"center");
    
    
    
    
    $fg = imagecreatefrompng("./img/fg.png");
    imagecopy($bg,$fg,0,0,0,0,imagesx($fg),imagesy($fg));
    
    // Duplizieren
    //imagecopy($bg,$bg,0,121,0,0,240,120);
    
    imagecopy($bg,$bg,0,120,0,0,240,120);
    imagecopy($bg,$bg,240-49-5,120+10,3,10,49,120);
    imagecopy($bg,$bg,3,120+10,52,10,183,106);
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


header("Content-type: image/png");
$size = filesize($imagefile);
header("Content-Transfer-Encoding: Binary"); 
header("Content-Length: ".$size);
readfile($imagefile);


?>