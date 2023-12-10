// var endpoint = "./faltblatt/"
var endpoint = "";

var configInit = {
    "folgezuege": "",
    "gleis": 3,
    "abschnitt": "",
    "intervalTime": 60,
    "timeFactor": 10,
    "stationNr": 8000115,
    "mode": 1,
    "zug1": {
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
   // set config to form
   $.get( endpoint+"default.json", function( data ) {
    // console.log(data);
    fillForm(data);
  });
}

function fillForm(config)
{
    $("#gleis").val(config.gleis);
    $("#vonnach").val(config.zug1.vonnach);
    $("#via").val(config.zug1.via);
    $("#zeit").val(config.zug1.zeit);
    $("#hinweis").val(config.zug1.hinweis);
    $("#abw").val(config.zug1.abw);
    $("#nr").val(config.zug1.nr);


    $("#vonnach2").val(config.zug2.vonnach);
    $("#via2").val(config.zug2.via);
    $("#zeit2").val(config.zug2.zeit);
    $("#hinweis2").val(config.zug2.hinweis);
    $("#abw2").val(config.zug2.abw);
    $("#nr2").val(config.zug2.nr);

    $("#vonnach3").val(config.zug3.vonnach);
    $("#via3").val(config.zug3.via);
    $("#zeit3").val(config.zug3.zeit);
    $("#hinweis3").val(config.zug3.hinweis);
    $("#abw3").val(config.zug3.abw);
    $("#nr3").val(config.zug3.nr);


    if (config.zug2.vonnach) 
    {
        $("#zug2").show();
    } else {
        $("#zug2").hide();
    }
    if (config.zug3.vonnach) 
    {
        $("#zug3").show();
    } else {
        $("#zug3").hide();
    }

}

function loadPic()
{
    console.log(configInit);
    config = JSON.parse(JSON.stringify(configInit));
    console.log(config);
    
    if ($("#gleis").val()) config.gleis = $("#gleis").val();
    if ($("#vonnach").val()) config.zug1.vonnach = $("#vonnach").val();
    if ($("#via").val()) config.zug1.via = $("#via").val();
    if ($("#zeit").val()) config.zug1.zeit = $("#zeit").val();
    if ($("#nr").val()) config.zug1.nr = $("#nr").val();
    if ($("#hinweis").val()) config.zug1.hinweis = $("#hinweis").val();
    else config.zug1.hinweis = "";
    if ($("#abw").val()) config.zug1.abw = $("#abw").val();
    else config.zug1.abw = 0;
    

    if ($("#vonnach2").val()) config.zug2.vonnach = $("#vonnach2").val();
    if ($("#via2").val()) config.zug2.via = $("#via2").val();
    if ($("#zeit2").val()) config.zug2.zeit = $("#zeit2").val();
    if ($("#nr2").val()) config.zug2.nr = $("#nr2").val();
    if ($("#hinweis2").val()) config.zug2.hinweis = $("#hinweis2").val();
    else config.zug2.hinweis = "";
    if ($("#abw2").val()) config.zug2.abw = $("#abw2").val();
    else config.zug2.abw = 0;

    if ($("#vonnach3").val()) config.zug3.vonnach = $("#vonnach3").val();
    if ($("#via3").val()) config.zug3.via = $("#via3").val();
    if ($("#zeit3").val()) config.zug3.zeit = $("#zeit3").val();
    if ($("#nr3").val()) config.zug3.nr = $("#nr3").val();
    if ($("#hinweis3").val()) config.zug3.hinweis = $("#hinweis3").val();
    else config.zug3.hinweis = "";
    if ($("#abw3").val()) config.zug3.abw = $("#abw3").val();
    else config.zug3.abw = 0;


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