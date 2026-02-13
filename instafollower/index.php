<?php

$username = "webfraggle";
// $url = "https://i.instagram.com/api/v1/users/web_profile_info/?username=".$username;
$url = "https://social-media-users-data-api-production.lightricks.workers.dev/instagram?username=".$username;
$hash = md5($url);
$cacheFile = "./cache/".$hash.".json";
$userAgent = "Instagram 76.0.0.15.395 Android (24/7.0; 640dpi; 1440x2560; samsung; SM-G930F; herolte; samsungexynos8890; en_US; 138226744)";
$reCache = false;

error_reporting(E_ALL);
ini_set("display_errors", 1);


include_once("gfx_functions.inc.php");
include_once("cors.inc.php");

cors();

$directory = "./cache";
if (!is_dir($directory)) {
    if (mkdir($directory, 0777, true)) {
        // echo "Directory created successfully.";
    } else {
        // echo "Failed to create directory.";
    }
}

if (!file_exists($cacheFile))
{
    $reCache = true;
} else
{
    /**
     * Überprüft, ob der Cache älter als 4 Stunden ist.
     * 
     * Der Cache lebt 4 Stunden (14400 Sekunden). Wenn die Differenz zwischen
     * der aktuellen Zeit und der letzten Änderungszeit der Cache-Datei
     * größer als 4 Stunden ist, wird der Cache als abgelaufen betrachtet
     * und sollte neu generiert werden.
     */
    if (time() - filemtime($cacheFile) > 4*60*60)
    {
        $reCache = true;
    }
}



if ($reCache)
{
    $options  = array('http' => array('user_agent' => $userAgent));
    $context  = stream_context_create($options);
    $response = file_get_contents($url, false, $context);
    file_put_contents($cacheFile, $response);
}

$jsonString = file_get_contents($cacheFile);
$data = json_decode($jsonString);

// print_r($data);
// print_r($data->followersCount);
// exit;
// $count = $data->data->user->edge_followed_by->count;
$count = $data->followersCount;



// print_r($data->data->user->edge_followed_by->count);

$fontBold = './fonts/Roboto/Roboto-Bold.ttf';
$fontRegular = './fonts/Roboto/Roboto-Regular.ttf';

$bg = imagecreatetruecolor(160, 160);
imagealphablending($bg, true);
imagesavealpha($bg, true);

$bgimg = imagecreatefrompng("./img/bg.png");
imagecopy($bg,$bgimg,0,0,0,0,imagesx($bgimg),imagesy($bgimg));
$x = 156;
$y = 63;
$s = 24;
addResizedTextToImage($count,$s,$fontBold,"#000000",1,1,$bg,$x+2,$y+2,"right");
addResizedTextToImage($count,$s,$fontBold,"#ffffff",1,1,$bg,$x,$y,"right");

$x = 155;
$y = 24;
$s = 8;
addResizedTextToImage("@".$username,$s,$fontRegular,"#000000",1,1,$bg,$x+2,$y+2,"right");
addResizedTextToImage("@".$username,$s,$fontRegular,"#ffffff",1,1,$bg,$x,$y,"right");

imagecopy($bg,$bg,0,80,0,0,160,80);


$imagefile = "./cache/".$hash.".png";
imagepng($bg, $imagefile);

header("Content-type: image/png");
$size = filesize($imagefile);
header("Content-Transfer-Encoding: Binary"); 
header("Content-Length: ".$size);
readfile($imagefile);

?>
