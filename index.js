var endpoint = "./faltblatt/"

$(function() {
    console.log( "ready!" );

    $("#sendBtn").click(loadPic);

});


function loadPic()
{
    obj = {};
    obj.zug1 = {};
    obj.zug1.ziel = $("#ziel").val();
    $.ajax({
        type: "POST",
        url: endpoint,
        xhrFields: {
            responseType: 'blob'
         },
        data: JSON.stringify(obj),
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