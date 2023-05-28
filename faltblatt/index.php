<?php
error_reporting(E_ALL);
ini_set("display_errors", 1);

include_once("gfx_functions.inc.php");
include_once("cors.inc.php");

cors();

$json = file_get_contents('php://input');
$cache = true;
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
        header("Content-type: image/png");
        readfile($imagefile);
        exit;
    }
}

$data = json_decode($json);
$font = './fonts/nimbus-sans-l/NimbusSanL-Bol.otf';


$bg = imagecreatetruecolor(240, 270);
imagealphablending($bg, true);
imagesavealpha($bg, true);

// add bg
$bgimg = imagecreatefrompng("./img/bg.png");
imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));

// Abfahrt
addResizedTextToImage("Abfahrt",10.5,$font,"#3a3c3b",0.5,1,$bg,57,30);

// Stunden und Minute
$time = explode(":",$data->zug1->zeit);
addResizedTextToImage($time[0],12.5,$font,"#3a3c3b",1,1,$bg,119,32, "right");
addResizedTextToImage($time[1],8.5,$font,"#3a3c3b",1,1,$bg,133,28, "center");

// Von Nach
$xpos = 54;
$vonnach = mb_convert_encoding(mb_strtoupper($data->zug1->vonnach), 'ISO-8859-1', 'UTF-8');
for ($i=0; $i < 13; $i++) { 
    $char =  substr($vonnach,$i,1);
    if ($char && $char != " ")
    {
        addResizedTextToImage($char,12,$font,"#3a3c3b",0.5,1,$bg,$xpos+5,119,"center");
    }
    $xpos += 14;
}

// vias
$vias = explode("-",$data->zug1->via);
$xpos = 147;
$ypos = 58;
$maxX = $xpos+87;
for ($i=0; $i < count($vias); $i++) { 
    $via = trim($vias[$i]);
    $width = addResizedTextToImage($via,9.5,$font,"#3a3c3b",0.5,1,$bg,$xpos,$ypos,"left",false);
    // print $via ." ". $width." ".$maxX." ".($xpos+$width);
    if ($xpos+$width < $maxX)
    {
        $paint = true;
    } else {
        if ($ypos == 58)
        {
            $ypos = 87;
            $xpos = 147;
            $paint = true;
        } else {
            $paint = false;
        }
    }
    if ($paint) addResizedTextToImage($via,9.5,$font,"#3a3c3b",0.5,1,$bg,$xpos,$ypos);
    $xpos += $width+2;
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
    imagecopy($bg,$fg,55,43,0,0,imagesx($fg),imagesy($fg));
}

// Gleis
addResizedTextToImage("Gleis",12,$font,"#00000",0.5,1,$bg,27,34,"center");

// Gleis Nr
addResizedTextToImage($data->gleis,33,$font,"#00000",0.5,1,$bg,27,110,"center");




$fg = imagecreatefrompng("./img/fg.png");
imagecopy($bg,$fg,0,0,0,0,imagesx($fg),imagesy($fg));


imagecopy($bg,$bg,0,135,0,0,240,135);
imagecopy($bg,$bg,240-49-5,135+10,3,10,49,120);
imagecopy($bg,$bg,3,135+10,52,10,183,120);


header("Content-type: image/png");

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

imagepng($bg);

?>