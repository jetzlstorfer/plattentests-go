
function applyBackgroundColor(element, imageurl) {
    if (element) {
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

            // Apply subtle gradient background to card
            let transparency = 0.1;
            let gradientColor = `rgba(${color.r}, ${color.g}, ${color.b}, ${transparency})`;
            
            // Apply to record card
            if (element.classList.contains('record-card')) {
                element.style.background = `linear-gradient(135deg, white 0%, ${gradientColor} 100%)`;
                
                // For dark mode
                if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
                    element.style.background = `linear-gradient(135deg, var(--dark-surface) 0%, rgba(${color.r}, ${color.g}, ${color.b}, 0.2) 100%)`;
                }
            } else {
                // Fallback for table rows
                element.style.backgroundColor = `rgba(${color.r}, ${color.g}, ${color.b}, ${transparency})`;
            }
            
            return color;
        };
    }
}

function applyTextColor(element, imageurl) {
    if (element) {
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
            var textColor = getOppositeColor(color);

            // For modern cards, we don't need to change text color as much
            // The design already handles contrast well
            if (element.classList.contains('record-card')) {
                // Only apply subtle adjustments if needed
                return color;
            } else {
                // Apply to table rows (fallback)
                element.style.color = textColor;

                // Apply the color to all links in the row
                var links = element.getElementsByTagName('a');
                for (var i = 0; i < links.length; i++) {
                    links[i].style.color = textColor;
                }
            }
            
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
