<?php
/*
 *  In a production environment, you probably want to be more restrictive, but this gives you
 *  the general idea of what is involved.  For the nitty-gritty low-down, read:
 *
 *  - https://developer.mozilla.org/en/HTTP_access_control
 *  - https://fetch.spec.whatwg.org/#http-cors-protocol
 *
 */
function timeToMinutes($time)
{
    $res = preg_match('/(^[\d]{1,2}):([\d]{1,2})/m', $time, $matches);
    if (!$res) return 0;
    $minutes = intval($matches[1]) * 60;
    $minutes += intval($matches[2]);
    return $minutes;
}
?>