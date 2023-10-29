// var endpoint = "./faltblatt/"
var endpoint = "";

var config = {
    "folgezuege": "",
    "gleis": 3,
    "abschnitt": "",
    "intervalTime": 60,
    "timeFactor": 10,
    "stationNr": 8000115,
    "mode": 1,
    "zug1": {
        "vonnach": "Fulda",
        "nr": "ICE123",
        "zeit": "12:30",
        "via": "Hanau - Gelnhausen - Neuhof - Traisbach",
        "abw": 0,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug2": {
        "vonnach": "",
        "nr": "",
        "zeit": "",
        "via": "",
        "abw": 0,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug3": {
        "vonnach": "",
        "nr": "",
        "zeit": "",
        "via": "",
        "abw": 0,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    }
}

var themes = {};

$(function() {
    console.log( "ready!" );
    $.get( "config.json", function( data ) {
        console.log(data);
        themes = data;
        createThemes();

      });

    $("#sendBtn").click(loadPic);
    $('#theme').change(themeChanged);
    $('#image').bind("load", imageLoaded);

});

function createThemes()
{
    $('#theme')
            .find('option')
            .remove()
        ;
        i=0;
        themes.themes.forEach(theme => {
        $('#theme').append($('<option>', {
            value: i,
            text: theme.title + ' - ' + theme.displaysize
        }));
        i++;
    });
    setTheme(0);
}

function imageLoaded(e)
{
    console.log('imageLoaded');
    var img = $('#image');
    var width = img.prop('naturalWidth');
    var height = img.prop('naturalHeight');
    img.width(width*2);
    img.height(height*2);
}
function themeChanged(e)
{
    console.log($('#theme').val());
    setTheme($('#theme').val());
}

function setTheme(nr)
{
    console.log("setTheme", nr);
    endpoint = themes.themes[nr].url;
    console.log(endpoint);
   $("#description").html(themes.themes[nr].description+"<br>URL: <b>"+window.location.href+endpoint.replace("./","")+"</b>"); 
}


function loadPic()
{
    if ($("#gleis").val()) config.gleis = $("#gleis").val();
    if ($("#vonnach").val()) config.zug1.vonnach = $("#vonnach").val();
    if ($("#via").val()) config.zug1.via = $("#via").val();
    if ($("#zeit").val()) config.zug1.zeit = $("#zeit").val();
    if ($("#nr").val()) config.zug1.nr = $("#nr").val();
    if ($("#hinweis").val()) config.zug1.hinweis = $("#hinweis").val();
    else config.zug1.hinweis = "";
    if ($("#abw").val()) config.zug1.abw = $("#abw").val();
    else config.zug1.abw = 0;
    
    $.ajax({
        type: "POST",
        url: endpoint,
        xhrFields: {
            responseType: 'blob'
         },
        data: JSON.stringify(config),
        contentType: "application/json;",
        // dataType: "blob",
        success: function(data){
            console.log(data);
            const url = window.URL || window.webkitURL;
            const src = url.createObjectURL(data);
            $('#image').attr('src', src);
        },
        error: function(errMsg) {
            console.error('getImage failed', errMsg);
        }
      });
}