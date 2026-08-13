document.addEventListener("DOMContentLoaded", function () {
  var form = document.querySelector("[data-lesson-editor-form]");
  if (!form) return;
  var sources = {
    teacher: document.getElementById("teacher-markdown"),
    student: document.getElementById("student-markdown")
  };
  var editors = null;
  if (!window.toastui || !window.toastui.Editor) {
    sources.teacher.classList.remove("markdown-source");
    sources.student.classList.remove("markdown-source");
    sources.teacher.style.minHeight = "500px";
    sources.student.style.minHeight = "500px";
  } else {
    editors = {
      teacher: new window.toastui.Editor({
        el: document.getElementById("teacher-editor"),
        height: "620px",
        initialEditType: "wysiwyg",
        previewStyle: "vertical",
        initialValue: sources.teacher.value,
        usageStatistics: false
      }),
      student: new window.toastui.Editor({
        el: document.getElementById("student-editor"),
        height: "620px",
        initialEditType: "wysiwyg",
        previewStyle: "vertical",
        initialValue: sources.student.value,
        usageStatistics: false
      })
    };
  }

  function syncMarkdown() {
    if (!editors) return;
    sources.teacher.value = editors.teacher.getMarkdown();
    sources.student.value = editors.student.getMarkdown();
  }

  function pdfBlob(base64) {
    var binary = window.atob(base64);
    var bytes = new Uint8Array(binary.length);
    for (var index = 0; index < binary.length; index++) {
      bytes[index] = binary.charCodeAt(index);
    }
    return new Blob([bytes], {type: "application/pdf"});
  }

  function downloadPDF(base64, filename) {
    var url = URL.createObjectURL(pdfBlob(base64));
    var link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  }

  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    syncMarkdown();
    var submit = form.querySelector('button[type="submit"]');
    var originalLabel = submit.textContent;
    submit.disabled = true;
    submit.textContent = "Đang xuất PDF…";
    try {
      var response = await fetch(form.action, {method: "POST", body: new FormData(form)});
      var result = await response.json();
      if (!response.ok) throw new Error(result.error || "Không thể xuất PDF");
      downloadPDF(result.teacher_pdf, result.teacher_filename);
      window.setTimeout(function () {
        downloadPDF(result.student_pdf, result.student_filename);
      }, 250);
    } catch (error) {
      window.alert(error.message || "Không thể xuất PDF");
    } finally {
      submit.disabled = false;
      submit.textContent = originalLabel;
    }
  });
  document.querySelectorAll("[data-download]").forEach(function (button) {
    button.addEventListener("click", function () {
      var type = button.getAttribute("data-download");
      var filename = type === "teacher" ? "GiaoVien.md" : "HocSinh.md";
      var markdown = editors ? editors[type].getMarkdown() : sources[type].value;
      var blob = new Blob([markdown], {type: "text/markdown;charset=utf-8"});
      var link = document.createElement("a");
      link.href = URL.createObjectURL(blob);
      link.download = filename;
      link.click();
      URL.revokeObjectURL(link.href);
    });
  });
});
