<?php

function addResizedTextToImage($text,$size,$font,$color,$xFactor,$yFactor,$im,$x,$y,$align="left",$paint=true,$linespacing=1)
{
    
    // print "\n---------\n";
    // print $text;
    // print "\n---------\n";
    $fontfactor = 96/72;
    // making an image double sized
    $scaleFactor = 4;
    $scaleDown = (1/$scaleFactor);
    $bbox = imageftbbox($size*$fontfactor*$scaleFactor, 0, $font, $text,array("linespacing" => $linespacing));
    $width = abs($bbox[0])+abs($bbox[2]);
    $height = abs($bbox[1])+abs($bbox[5]);
    if ($width <= 0 || $height <= 0) return 0;
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

    imagefttext($textImage, $size*$fontfactor*$scaleFactor, 0, $bbox[0], $height-$bbox[1]+$offset, $fontColor, $font, $text,array("linespacing" => $linespacing));
    
    
    // print "\n ";
    // print " imgWidth:";
    // print_r(imagesx($textImage));
    // print " imgHeight:";
    // print_r(imagesy($textImage));
    // print "\n ";
    
    $targetWidth = ceil($width*$scaleDown*$xFactor);
    $targetHeight = ceil(($height+$offset)*$scaleDown*$yFactor);
    if ($paint)
    {
        switch ($align) {
            case 'right':
                imagecopyresampled($im, $textImage, $x-$targetWidth, $y-ceil(($height-$bbox[1])*$scaleDown), 0, 0, $targetWidth, ceil(($height+$offset)*$scaleDown*$yFactor), $width, $height+$offset);
                break;
            case 'center':
                imagecopyresampled($im, $textImage, round($x-($targetWidth*$scaleDown)), $y-ceil(($height-$bbox[1])*$scaleDown), 0, 0, $targetWidth, ceil(($height+$offset)*$scaleDown*$yFactor), $width, $height+$offset);
                break;
            
            case 'bottom-left':
                imagecopyresampled($im, $textImage, $x, $y-$targetHeight, 0, 0, $targetWidth, $targetHeight, $width, $height+$offset);
            break;
            
            case 'top-left':
            default:
                imagecopyresampled($im, $textImage, $x, $y, 0, 0, $targetWidth, $targetHeight, $width, $height+$offset);
            break;
        }
    }
    return $targetWidth;
}

function wrapText($string, $fontFace, $fontSize, $width){
    $fontfactor = 96/72;
    $ret = "";
    $string = str_replace("-","- ",$string);
    $arr = explode(' ', $string);

    foreach ( $arr as $word ){

        $teststring = $ret.' '.$word;
        // print "\n------";
        // print_r($teststring);
        // print "\n";
        $testbox = imagettfbbox($fontSize*$fontfactor, 0, $fontFace, $teststring);
        // print_r($testbox);
        $word = str_replace("- ","-",$word);
        if ( $testbox[2] > $width ){
            $ret.=($ret==""?"":"\n").$word;
        } else {
            if (str_ends_with($ret,"-"))
            {
                $ret.=($ret==""?"":'').$word;
            } else {
                $ret.=($ret==""?"":' ').$word;
            }
        }
    }

    return $ret;
}

?>