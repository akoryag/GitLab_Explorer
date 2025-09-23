// Переключение группы
function toggleGroup(id) {
  const content = document.getElementById(id);
  const btn = document.getElementById("btn-" + id);
  const isHidden = content.style.display === "none" || !content.style.display;
  content.style.display = isHidden ? "block" : "none";
  btn.textContent = isHidden ? "[−]" : "[+]";
}

// Массовый выбор ref
function applyGroupRef(groupIdx) {
  const groupSelect = document.getElementById("group-ref-" + groupIdx);
  const selectedRef = groupSelect.value;
  if (!selectedRef) return;

  const projectSelects = document.querySelectorAll(
    `[id^="ref-${groupIdx}-"]`
  );
  projectSelects.forEach((sel) => {
    if ([...sel.options].some((opt) => opt.value === selectedRef)) {
      sel.value = selectedRef;
    }
  });
}

// CSS-класс по статусу
function getStatusClass(status) {
  const map = {
    failed: "status-btn status-failed",
    manual: "status-btn status-manual",
    success: "status-btn status-success",
    running: "status-btn status-running",
  };
  return map[status] || "status-btn";
}

// Кнопки действий
function renderJobButtons(groupIdx, projIdx, projectId, job) {
  const makeBtn = (cls, text, action) =>
    `<button class="${cls}" onclick="${action}">${text}</button>`;

  let buttons = "";
  if (job.status === "manual") {
    buttons += makeBtn(
      "run-btn",
      "Запустить",
      `playJob(${groupIdx},${projIdx},${projectId},${job.id})`
    );
  }
  if (["failed", "success", "canceled"].includes(job.status)) {
    buttons += makeBtn(
      "retry-btn",
      "Перезапустить",
      `retryJob(${groupIdx},${projIdx},${projectId},${job.id})`
    );
  }
  if (["running", "pending"].includes(job.status)) {
    buttons += makeBtn(
      "cancel-btn",
      "Отменить",
      `cancelJob(${groupIdx},${projIdx},${projectId},${job.id})`
    );
  }
  return buttons;
}

// Универсальный вызов API
async function jobAction(action, groupIdx, projIdx, projectId, jobId) {
  try {
    const resp = await fetch(
      `/job?project_id=${projectId}&job_id=${jobId}&action=${action}`,
      { method: "POST" }
    );
    if (!resp.ok) throw new Error("HTTP error " + resp.status);

    const data = await resp.json();
    if (data.error) {
      alert("Ошибка: " + data.error);
    } else {
      alert(`Джоба: ${action} успешно!`);
      loadPipeline(groupIdx, projIdx, projectId);
    }
  } catch (err) {
    alert("Ошибка: " + err.message);
  }
}

const playJob = (...args) => jobAction("play", ...args);
const retryJob = (...args) => jobAction("retry", ...args);
const cancelJob = (...args) => jobAction("cancel", ...args);

// Загрузка пайплайна
async function loadPipeline(groupIdx, projIdx, projectId) {
  const select = document.getElementById(`ref-${groupIdx}-${projIdx}`);
  const ref = select.value;
  if (ref === "—") return alert("Выберите реф");

  const section = document.getElementById(`pipeline-${groupIdx}-${projIdx}`);
  const errorP = document.getElementById(
    `pipeline-error-${groupIdx}-${projIdx}`
  );
  const jobsTbody = document.getElementById(`jobs-${groupIdx}-${projIdx}`);

  section.style.display = "block";
  errorP.style.display = "none";
  jobsTbody.innerHTML = "";

  try {
    const resp = await fetch(
      `/pipeline?project_id=${projectId}&ref=${encodeURIComponent(ref)}`
    );
    if (!resp.ok) throw new Error("HTTP error " + resp.status);

    const data = await resp.json();
    if (data.error) {
      errorP.textContent = data.error;
      errorP.style.display = "block";
      return;
    }

    const stages = {};
    (data.jobs || []).forEach((job) => {
      (stages[job.stage] ||= []).push(job);
    });

    // обычные jobs
    Object.entries(stages).forEach(([stage, jobs]) => {
      jobsTbody.insertAdjacentHTML(
        "beforeend",
        `<tr class="stage-row"><td><b>Stage:</b> ${stage}</td></tr>`
      );
      jobs.forEach((job) => {
        jobsTbody.insertAdjacentHTML(
          "beforeend",
          `<tr><td style="padding-left:20px;">
            <div class="job-line">
              <span class="job-name">${job.name}</span>
              <button class="${getStatusClass(job.status)}">${job.status}</button>
              ${renderJobButtons(groupIdx, projIdx, projectId, job)}
            </div>
          </td></tr>`
        );
      });
    });

    // bridges
    (data.bridges || []).forEach((bridge) => {
      jobsTbody.insertAdjacentHTML(
        "beforeend",
        `<tr class="stage-row"><td><b>Stage:</b> ${bridge.name}</td></tr>`
      );
      if (bridge.downstream_jobs?.length) {
        const bStages = {};
        bridge.downstream_jobs.forEach(
          (job) => (bStages[job.stage] ||= []).push(job)
        );
        Object.entries(bStages)
          .reverse()
          .forEach(([stage, jobs]) => {
            jobsTbody.insertAdjacentHTML(
              "beforeend",
              `<tr class="stage-row"><td style="padding-left:10px;"><b>Stage:</b> ${stage}</td></tr>`
            );
            jobs.forEach((job) => {
              jobsTbody.insertAdjacentHTML(
                "beforeend",
                `<tr><td style="padding-left:30px;">
                  <div class="job-line">
                    <span class="job-name">${job.name}</span>
                    <button class="${getStatusClass(job.status)}">${job.status}</button>
                    ${renderJobButtons(groupIdx, projIdx, projectId, job)}
                  </div>
                </td></tr>`
              );
            });
          });
      }
    });

    filterJobs(groupIdx);
  } catch (err) {
    errorP.textContent = "Ошибка: " + err.message;
    errorP.style.display = "block";
  }
}

// Фильтрация jobs
function filterJobs(groupIdx) {
  const input = document.getElementById("searchJobs-" + groupIdx);
  if (!input) return;
  const search = input.value.trim().toLowerCase();

  document.querySelectorAll(`[id^="jobs-${groupIdx}-"]`).forEach((tbody) => {
    const rows = Array.from(tbody.querySelectorAll("tr"));
    let currentStage = null;
    let hasVisibleJob = false;

    rows.forEach((row) => {
      if (row.classList.contains("stage-row")) {
        if (currentStage) {
          currentStage.style.display = hasVisibleJob ? "" : "none";
        }
        currentStage = row;
        hasVisibleJob = false;
        row.style.display = "";
      } else {
        const name = row.querySelector(".job-name")?.textContent.toLowerCase();
        const matches = !search || (name && name.includes(search));
        row.style.display = matches ? "" : "none";
        if (matches) hasVisibleJob = true;
      }
    });

    if (currentStage) {
      currentStage.style.display = hasVisibleJob ? "" : "none";
    }
  });
}
