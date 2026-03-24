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
    $fontBold = './fonts/Roboto/RobotoCondensed-Bold.ttf';
    $fontRegular = './fonts/Roboto/RobotoCondensed-Regular.ttf';
    
    $bg = imagecreatetruecolor(144, 144);
    imagealphablending($bg, true);
    imagesavealpha($bg, true);
    

    // add bg
    $bgimg = imagecreatefrompng("./img/bg.png");
    imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
    
    

    $vonnach = mb_convert_encoding($data->zug1->vonnach, 'ISO-8859-1', 'UTF-8');
    $text = wrapText($vonnach,$fontRegular,18,132);


    addResizedTextToImage($text,18,$fontRegular,"#ffffff",1,1,$bg,8,78);
    addResizedTextToImage($text,18,$fontRegular,"#ffffff",1,1,$bg,8,78);


    // Gleis
    $gleis = $data->gleis;
    addResizedTextToImage($gleis,41.25,$fontBold,"#ffffff",1,1,$bg,4,15,"left");

    $tt = $data->zug1->zeit;
    addResizedTextToImage($tt,22,$fontBold,"#ffffff",1,1,$bg,135,54,"right");
    
    $tt = $data->zug1->nr;
    addResizedTextToImage($tt,15,$fontBold,"#ffffff",1,1,$bg,135,30,"right");

    



// exit();


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