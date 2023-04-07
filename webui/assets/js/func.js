
function applyBackgroundColor(row, imageurl) {
    if (row) {
        // Create a new image element
        var img = new Image();

        // Set the image source to the specified URL
        img.src = imageurl;
		img.src = imageurl + '?' + new Date().getTime();
		// send image through google proxy to avoid CORS issues
		let googleProxyURL = 'https://images1-focus-opensocial.googleusercontent.com/gadgets/proxy?container=focus&refresh=2592000&url=';
		img.setAttribute('crossOrigin', 'Anonymous');
		img.src = googleProxyURL + encodeURIComponent(imageurl);
		
        // Wait for the image to load
        img.onload = function() {
            // Calculate the average color of the image
            var color = getAverageColor(img);

            // Apply the color to the background of the row
            let transparency = 0.9;
            row.style.backgroundColor = "rgb(" + color.r + " " + color.g + " " + color.b + " / " + transparency +")";
            return color;
        };
    }
}

function applyTextColor(row, imageurl) {
    if (row) {
        // Create a new image element
        var img = new Image();

        // Set the image source to the specified URL
        img.src = imageurl;
        img.src = imageurl + '?' + new Date().getTime();
        // send image through google proxy to avoid CORS issues
        let googleProxyURL = 'https://images1-focus-opensocial.googleusercontent.com/gadgets/proxy?container=focus&refresh=2592000&url=';
        img.setAttribute('crossOrigin', 'Anonymous');
        img.src = googleProxyURL + encodeURIComponent(imageurl);
        
        // Wait for the image to load
        img.onload = function() {
            // Calculate the average color of the image
            var color = getAverageColor(img);

            // Apply the color to the background of the row
            row.style.color = getOppositeColor(color);
            return color;
        };
    }
}


function getAverageColor(img) {
    // Create a canvas element
    var canvas = document.createElement("canvas");

    // Set the canvas dimensions to match the image dimensions
    canvas.width = img.width;
    canvas.height = img.height;

    // Draw the image onto the canvas
    var ctx = canvas.getContext("2d");
    ctx.drawImage(img, 0, 0);

    // Get the pixel data from the canvas
    var data = ctx.getImageData(0, 0, canvas.width, canvas.height);

    // Calculate the average color of the image
    var length = data.data.length;
    var rgb = {r:0,g:0,b:0};
    var count = 0;

    for (var i = 0; i < length; i += 4) {
        if (data.data[i+3] > 0) {
            rgb.r += data.data[i];
            rgb.g += data.data[i+1];
            rgb.b += data.data[i+2];
            count++;
        }
    }

    // ~~ used to floor values
    rgb.r = ~~(rgb.r/count);
    rgb.g = ~~(rgb.g/count);
    rgb.b = ~~(rgb.b/count);

    return rgb;
}



function getOppositeColor(color) {
    var r = color.r;
    var g = color.g;
    var b = color.b;
    var yiq = ((r*299)+(g*587)+(b*114))/1000;
    return (yiq >= 128) ? 'black' : 'white';
}
