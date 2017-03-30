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
      window.location.replace(location.protocol + "//" + location.host + "/welcome/");
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
              'Are you sure that you want to create the virtual domain <b>' +
              domain_name +
              '</b>? This will delete all users and aliases for this domain as well!',
              function(result) {
                if(result) {
                  delete_domain(domainID)
                }
              }
            )
          });
}

function fill_domains() {
  var spinner = new Spinner().spin();
  document.getElementById('virtual-domains').appendChild(spinner.el);
  $('#get-alert-status').addClass('hidden');
  data_table.clear();
  var destination = location.protocol + "//" + location.host + "/listdomains";
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
              var button_td = $('<td></td>').append(button);
              var jqueryRow = $('<tr></tr>').append( $('<td></td>').text(domain_name), button_td );
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
