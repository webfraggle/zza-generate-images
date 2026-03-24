<?php

function addResizedTextToImage($text,$size,$font,$color,$xFactor,$yFactor,$im,$x,$y,$align="left",$paint=true)
{
    $fontfactor = 96/72;
    // making an image double sized
    $bbox = imageftbbox($size*$fontfactor*2, 0, $font, $text);
    $width = abs($bbox[0])+abs($bbox[2]);
    $height = abs($bbox[1])+abs($bbox[5]);
    // print $width;
    // print $height;
    // print_r($bbox);
    // exit;
    $offset = 1;
    $textImage = imagecreatetruecolor($width, $height+$offset);
    imagealphablending($textImage, false);
    imagesavealpha($textImage, true);

    list($r, $g, $b) = sscanf($color, "#%02x%02x%02x");
    $fontColor = imagecolorallocate($im, $r, $g, $b);
    $bg=imagecolorallocatealpha($im,0,0,0,127);
    imagefill($textImage, 0, 0, $bg);

    imagefttext($textImage, $size*$fontfactor*2, 0, $bbox[0], $height-$bbox[1]+$offset, $fontColor, $font, $text);
    $targetWidth = ceil($width*0.5*$xFactor);
    if ($paint)
    {
        switch ($align) {
            case 'right':
                imagecopyresampled($im, $textImage, $x-$targetWidth, $y-ceil(($height-$bbox[1])*0.5), 0, 0, $targetWidth, ceil(($height+$offset)*0.5*$yFactor), $width, $height+$offset);
                break;
            case 'center':
                imagecopyresampled($im, $textImage, round($x-($targetWidth*0.5)), $y-ceil(($height-$bbox[1])*0.5), 0, 0, $targetWidth, ceil(($height+$offset)*0.5*$yFactor), $width, $height+$offset);
                break;
            
            default:
                imagecopyresampled($im, $textImage, $x, $y-ceil(($height-$bbox[1])*0.5), 0, 0, $targetWidth, ceil(($height+$offset)*0.5*$yFactor), $width, $height+$offset);
                break;
        }
    }
    return $targetWidth;
}
?>