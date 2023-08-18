

function calculateDaysBetweenDates(date1, date2) {
    var oneDay = 24 * 60 * 60 * 1000; // hours*minutes*seconds*milliseconds
    var firstDate = date1;
    var secondDate = date2;

    var diffDays = Math.round(Math.abs((firstDate.getTime() - secondDate.getTime()) / (oneDay)));
    return diffDays;
}

// find all images without alt-text and add a blue border
function findImagesWithoutAltText() {
    var images = document.getElementsByTagName('img');
    for (var i = 0; i < images.length; i++) {
        if (!images[i].alt) {
            images[i].style.border = "5px solid blue";
        }
    }
}

