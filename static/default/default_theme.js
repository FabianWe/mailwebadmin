/*!
The MIT License (MIT)

Copyright (c) 2017 Fabian Wenzelmann

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/


function post_login() {
  // first post the data
  var destination = location.protocol + "//" + location.host + "/login/";
  var form_data = $('#login-credentials').serializeArray();
  form_map = { 'username': form_data[1]['value'],
    'password': form_data[2]['value'] }
  form_map['remember-me'] = (form_data.length == 4)
  var json_data = JSON.stringify(form_map);
  var jqxhr = $.ajax({
    type: 'POST',
    url: destination,
    data: json_data,
    headers: {
      "X-CSRF-Token": form_data[0]['value'],
    },
    success: function(data, status) {
      window.location.replace(location.protocol + "//" + location.host + "/");
    }
  })
  .fail(function(jqXHR, textStatus, error) {
     $('#login-status').addClass('alert-danger').removeClass('alert-info').html("Authentication error, username / password wrong.");
  });
}

function delete_confirm(title, message, callback) {
  bootbox.confirm({
    title: title,
    message: message,
    callback: callback,
    buttons: {
      cancel: {
        label: '<span class="glyphicon glyphicon-remove-circle"/> Cancel',
        className: 'btn-danger'
      },
      confirm: {
        label: '<span class="glyphicon glyphicon-ok-circle"/> Delete',
        className: 'btn-success'
      }
    }
  });
}

function set_alert(alert_obj, status, html) {
  if(status == 'success') {
    return alert_obj.removeClass('hidden alert-danger').addClass("alert-success").html(html);
  } else {
    return alert_obj.removeClass('hidden alert-success').addClass("alert-danger").html(html);
  }
}


function add_domain() {
  var spinner = new Spinner().spin();
  document.getElementById('virtual-domains').appendChild(spinner.el);
  var destination = location.protocol + "//" + location.host + "/listdomains/";
  var form_data = $('#add-domain-form').serializeArray();
  form_map = { 'domain-name': form_data[0]['value'] }
  var json_data = JSON.stringify(form_map);
  var jqxhr = $.ajax({
    type: 'POST',
    url: destination,
    data: json_data,
    headers: {
      "X-CSRF-Token": csrf_listdomains,
    },
    success: function(data, status) {
      set_alert($('#manipulate-alert-status'), 'success', 'Added new virtual domain');
    }
  })
  .fail(function(jqXHR, textStatus, error) {
    set_alert($('#manipulate-alert-status'), 'error', 'Error adding domain: ' + error);
  })
  .always(function() {
    spinner.stop();
    fill_domains();
  });
}


function delete_domain(domainID) {
  var spinner = new Spinner().spin();
  document.getElementById('virtual-domains').appendChild(spinner.el);
  var destination = location.protocol + "//" + location.host + "/listdomains/" + domainID + "/";
  var jqxhr = $.ajax({
    type: "DELETE",
    url: destination,
    headers: {
        "X-CSRF-Token": csrf_listdomains,
    },
    success: function(data, status) {
      set_alert($('#manipulate-alert-status'), 'success', 'Successfully removed domain');
    }
  })
  .fail(function(jqXHR, textStatus, error) {
    set_alert($('#manipulate-alert-status'), 'error', 'Error removing domain: ' + error);
  })
  .always(function() {
    fill_domains();
    spinner.stop();
  });
}

function remove_domain_button(domain_name, domainID) {
  return $('<button type="button" class="btn btn-default"></button>')
          .append( $('<span class="glyphicon glyphicon-remove" style="color:red"></span>') )
          .click(function() {
            delete_confirm('Delete Virtual Domain?',
              'Are you sure that you want to delete the virtual domain <b>' +
              domain_name +
              '</b>? This will delete all users and aliases for this domain as well!' +
              '<p/>Maybe also all the emails for this domain.',
              function(result) {
                if(result) {
                  delete_domain(domainID);
                }
              }
            )
          });
}

function delete_user(userID) {
  var spinner = new Spinner().spin();
  document.getElementById('virtual-users').appendChild(spinner.el);
  var destination = location.protocol + "//" + location.host + "/listusers/" + userID + "/";
  var jqxhr = $.ajax({
    type: "DELETE",
    url: destination,
    headers: {
        "X-CSRF-Token": csrf_listusers,
    },
    success: function(data, status) {
      set_alert($('#manipulate-alert-status'), 'success', 'Successfully removed user');
    }
  })
  .fail(function(jqXHR, textStatus, error) {
    set_alert($('#manipulate-alert-status'), 'error', 'Error removing user: ' + error);
  })
  .always(function() {
    fill_users();
    spinner.stop();
  });
}

function remove_user_button(user_mail, user_id) {
  return $('<button type="button" class="btn btn-default"></button>')
          .append( $('<span class="glyphicon glyphicon-remove" style="color:red"></span>') )
          .click(function() {
            delete_confirm('Delete Virtual User?',
              'Are you sure that you want to delete the virtual user <b>' +
              user_mail +
              '</b>? This may also delete all emails for this user!' +
              '<p/>Also you should check your aliases (was some email forwarded to this user?).',
              function(result) {
                if(result) {
                  delete_user(user_id);
                }
              }
            )
          });
}

function change_password(user_id, password) {
  if (password.length < 6) {
    bootbox.alert("Password must be at least six characters long")
    return
  }
  var spinner = new Spinner().spin();
  document.getElementById('virtual-users').appendChild(spinner.el);
  var destination = location.protocol + "//" + location.host + "/listusers/" + user_id + "/";
  var jqxhr = $.ajax({
    type: "UPDATE",
    url: destination,
    data: JSON.stringify( { "password": password } ),
    headers: {
        "X-CSRF-Token": csrf_listusers,
    },
    success: function(data, status) {
      set_alert($('#manipulate-alert-status'), 'success', 'Successfully changed password');
    }
  })
  .fail(function(jqXHR, textStatus, error) {
    set_alert($('#manipulate-alert-status'), 'error', 'Error changing password: ' + error);
  })
  .always(function() {
    spinner.stop();
  });
}

function change_password_button(user_mail, user_id) {
  return $('<button type="button" class="btn btn-default"></button>')
          .append( $('<span class="glyphicon glyphicon-lock" style="color:teal"></span>') )
          .click(function() {
            bootbox.prompt({
              title: "Change Password for <b>" + escapeHtml(user_mail) + "</b>",
              inputType: 'password',
              callback: function (result) {
                if (result === null) {
                  bootbox.alert("Password not changed")
                }
                else {
                  change_password(user_id, result);
                }
              }
            });
          });
}

function fill_domains() {
  var spinner = new Spinner().spin();
  document.getElementById('virtual-domains').appendChild(spinner.el);
  $('#get-alert-status').addClass('hidden');
  data_table.clear();
  var destination = location.protocol + "//" + location.host + "/listdomains/";
  var jqxhr = $.ajax({
    type: "GET",
    url: destination,
    data: "",
    success: function(data, status, request) {
      csrf_listdomains = request.getResponseHeader("X-CSRF-Token");
      if(data) {
        try {
          var jsonDecoded = JSON.parse(data);
          for(var domainID in jsonDecoded) {
            if(jsonDecoded.hasOwnProperty(domainID)) {
              var domain_name = jsonDecoded[domainID];
              var button = remove_domain_button(domain_name, domainID)
              var button_td = $('<td class="datatable-button"></td>').append(button);
              var jqueryRow = $('<tr></tr>').append( $('<td></td>').html('<a href="/users?domain=' + domainID + '">' + escapeHtml(domain_name) + "</a>"), button_td );
              data_table.row.add(jqueryRow);
            }
          }
        }
        catch(e) {
          set_alert($('#get-alert-status'), 'error', 'Error getting domain list: Invalid return syntax');
        }
      }
    }
  }).fail(function(jqXHR, textStatus, error) {
    set_alert($('#get-alert-status'), 'error', 'Error getting domain list: ' + error);
  })
  .always(function() {
    data_table.draw();
    spinner.stop();
  });
}

function add_user() {
  var spinner = new Spinner().spin();
  document.getElementById('virtual-users').appendChild(spinner.el);
  var destination = location.protocol + "//" + location.host + "/listusers";
  var form_data = $('#add-user-form').serializeArray();
  form_map = { 'mail': form_data[0]['value'], 'password': form_data[1]['value'] }
  if (form_data[1]['value'].length < 6) {
    bootbox.alert("Password must be at least six characters long")
    return
  }
  var json_data = JSON.stringify(form_map);
  var jqxhr = $.ajax({
    type: 'POST',
    url: destination,
    data: json_data,
    headers: {
      "X-CSRF-Token": csrf_listusers,
    },
    success: function(data, status) {
      set_alert($('#manipulate-alert-status'), 'success', 'Added new user');
    }
  })
  .fail(function(jqXHR, textStatus, error) {
    set_alert($('#manipulate-alert-status'), 'error', 'Error adding user: ' + error);
  })
  .always(function() {
    spinner.stop();
    fill_users();
  });
}

function fill_users() {
  var domainID = "-1";
  var urlParam = getUrlParameter('domain')
  if (typeof urlParam != 'undefined') {
    domainID = urlParam
  }
  var spinner = new Spinner().spin();
  document.getElementById('virtual-users').appendChild(spinner.el);
  $('#get-alert-status').addClass('hidden');
  data_table.clear();
  var destination = location.protocol + "//" + location.host + "/listusers" + "?domain=" + domainID;
  var jqxhr = $.ajax({
    type: "GET",
    url: destination,
    data: "",
    success: function(data, status, request) {
      csrf_listusers = request.getResponseHeader("X-CSRF-Token");
      if(data) {
        try {
          var jsonDecoded = JSON.parse(data);
          for(var mail in jsonDecoded) {
            if (jsonDecoded.hasOwnProperty(mail)) {
              var entry = jsonDecoded[mail];
              var aliases = [];
              var aliasDict = entry['AliasFor'];
              for (var aliasEntry in aliasDict) {
                if (aliasDict.hasOwnProperty(aliasEntry)) {
                  aliases.push(aliasDict[aliasEntry]["Dest"]);
                }
              }
              var virtual_user = entry["VirtualUser"];
              if (virtual_user) {
                var virtualUserID = entry["VirtualUserID"];
                var jqueryRow = $('<tr></tr>')
                  .append( $('<td class="virtual-user"></td>').text(mail) )
                  .append( $('<td></td>').text(aliases.join(', ')) )
                  .append( $('<td class="datatable-button"></td>').html(change_password_button(mail, virtualUserID)) )
                  .append( $('<td class="datatable-button"></td>').html(remove_user_button(mail, virtualUserID)) );
                data_table.row.add(jqueryRow);
              } else {
                var jqueryRow = $('<tr></tr>')
                  .append( $('<td class="only-alias"></td>').text(mail) )
                  .append( $('<td></td>').text(aliases.join(', ')) )
                  .append( $('<td></td>') )
                  .append( $('<td></td>') );
                data_table.row.add(jqueryRow);
              }
            }
          }
        }
        catch(e) {
          set_alert($('#get-alert-status'), 'error', 'Error getting users list: Invalid return syntax');
        }
      }
    }
  }).fail(function(jqXHR, textStatus, error) {
    set_alert($('#get-alert-status'), 'error', 'Error getting user list: ' + error);
  })
  .always(function() {
    data_table.draw();
    spinner.stop();
  });
}

// next stuff is from https://github.com/janl/mustache.js/blob/master/mustache.js
// TODO use this more!
var entityMap = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
  '/': '&#x2F;',
  '`': '&#x60;',
  '=': '&#x3D;'
};

function escapeHtml (string) {
  return String(string).replace(/[&<>"'`=\/]/g, function (s) {
    return entityMap[s];
  });
}

// next function from http://www.jquerybyexample.net/2012/06/get-url-parameters-using-jquery.html
// adjusted, however
function getUrlParameter(sParam) {
  var sPageURL = decodeURIComponent(window.location.search.substring(1)),
      sURLVariables = sPageURL.split('&'),
      sParameterName,
      i;

  for (i = 0; i < sURLVariables.length; i++) {
      sParameterName = sURLVariables[i].split('=');

      if (sParameterName[0] === sParam) {
          return sParameterName[1] === undefined ? true : sParameterName[1];
      }
  }
};

/* The following code was taken from
http://bootsnipp.com/snippets/featured/fancy-sidebar-navigation */

$(document).ready(function () {
  var trigger = $('.hamburger'),
      overlay = $('.overlay'),
     isClosed = false;

    trigger.click(function () {
      hamburger_cross();
    });

    function hamburger_cross() {

      if (isClosed == true) {
        overlay.hide();
        trigger.removeClass('is-open');
        trigger.addClass('is-closed');
        isClosed = false;
      } else {
        overlay.show();
        trigger.removeClass('is-closed');
        trigger.addClass('is-open');
        isClosed = true;
      }
  }

  $('[data-toggle="offcanvas"]').click(function () {
        $('#wrapper').toggleClass('toggled');
  });
});

/* end code from
http://bootsnipp.com/snippets/featured/fancy-sidebar-navigation */
