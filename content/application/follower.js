function refreshFollower(){
    var date = new Date();
    var offset = date.getTimezoneOffset()/60+1;
    var len = offset.toString().length
    if(offset > 0){
        if(len == 2){
            var str = "-";
            offset = str.concat(offset).concat(":00");
        }
        else{
            var str = "-0";
            offset = str.concat(offset).concat(":00");
        }
    }
    if(offset <= 0){
        if(len == 2){
            var str = "+";
            offset = str.concat(offset).concat(":00");
        }
        else{
            var str = "+0";
            offset = str.concat(offset).concat(":00");
        }
    }
    var microseconds =date % 1000000/ 1000000;
    var iso = date.toISOString();
    iso = iso.split(".")[0].concat(".",(microseconds.toString().split(".")[1])).concat(offset);
    $.ajax({
        url: "/socialnetwork/refresh-follower?last_refresh="+iso,
        dataType : "json",
        success: updateList
    });
}

function updateComment(id, comments, userSet) {
    $(comments).each(function() {
        if(this.fields.post == id){
            if(!$("#comment_content_"+id).children("#comment_"+this.pk).length){   
                $("#comment_content_"+id).append(
                    "<div class=comment id=comment_"+this.pk+">Comment by <a id=id_comment_profile_"+this.pk+" href=profile/"+this.fields.user+">"+userSet[this.fields.user-1].first_name+" "+userSet[this.fields.user-1].last_name+"</a> <span id=id_comment_text_"+this.pk+">"+sanitize(this.fields.comment_text)+"</span> <span id=id_comment_date_time_"+this.pk+">"+this.fields.time+"</span></div>"
                )
            };
        }  
    });
}


function updateList(items) {
    var userSet = []
    $(items).each(function() {
        if(this.model.split(".")[1]=="user"){
            userSet.push({'first_name':this.fields.first_name,'last_name':this.fields.last_name})
        }
    })
    console.log(userSet)
    $(items).each(function() {
        if(this.model.split(".")[1]=="post"){
            if(!$("#post_"+this.pk).length){
                $("#post_content").prepend(
                    "<tr><td id=post_"+this.pk+"><span class=post font-italic>Post by <a id=id_post_profile_"+this.pk+" href=profile/"+this.fields.user+"> "+userSet[this.fields.user-1].first_name+" "+userSet[this.fields.user-1].last_name+" </a></span><span id=id_post_text_"+this.pk+"> "+sanitize(this.fields.post_text)+" </span><span class=font-italic id=id_post_date_time_"+this.pk+">"+this.fields.time+"</span></td></tr><tr><td id=comment_content_"+this.pk+"></td></tr><tr><td><div class=comment><td>Comment: </td><td><input name=comment_text_input id=id_comment_text_input_"+this.pk+"></td><td><button onclick=addComment("+this.pk+") type=submit name=button value=comment id=id_comment_button_"+this.pk+">Submit</button></td></div>"
                );
            };
        }
        if(this.model.split(".")[1]=="comment"){
            updateComment(this.fields.post, this, userSet)
        }
    })
}

// urienconded
function sanitize(s) {
    // Be sure to replace ampersand first
    return s.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
}

function displayError(message) {
    $("#error").html(message);
}

function getCSRFToken() {
    var cookies = document.cookie.split(";");
    for (var i = 0; i < cookies.length; i++) {
        c = cookies[i].trim();
        if (c.startsWith("csrftoken=")) {
            return c.substring("csrftoken=".length, c.length);
        }
    }
    return "unknown";
}

function addComment(id) {
    var commentElement = $("#id_comment_text_input_"+id);
    var commentValue   = commentElement.val();
    // var encoded = encodeURI(commentValue)

    // Clear input box and old error message (if any)
    commentElement.val('');
    displayError('');

    $.ajax({
        url: "/socialnetwork/add-comment/"+id,
        type: "POST",
        data: "comment_text_input="+commentValue+"&csrfmiddlewaretoken="+getCSRFToken(),
        dataType : "json",
        success: function(response) {
            var userSet = []
            var comments = []
            $(response).each(function() {
                if(this.model.split(".")[1]=="user"){
                    userSet.push({'first_name':this.fields.first_name,'last_name':this.fields.last_name})
                }
            })
            $(response).each(function() {
                if(this.model.split(".")[1]=="comment"){
                    comments.push(this)
                }
            })
            if (Array.isArray(response)) {
                updateComment(id, comments, userSet);
            } else {
                displayError(response.error);
            }
        }
    });
}
// The index.html does not load the list, so we call getList()
// as soon as page is finished loading
window.onload = refreshFollower;

// causes list to be re-fetched every 5 seconds
window.setInterval(refreshFollower, 5000);