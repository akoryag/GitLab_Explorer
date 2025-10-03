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

function toggleAllProjects(groupIdx) {
  const headerCheckbox = document.getElementById(`header-checkbox-${groupIdx}`);
  const projectCheckboxes = document.querySelectorAll(`.project-checkbox[data-group="${groupIdx}"]`);
  
  const isChecked = headerCheckbox.checked;
    
  projectCheckboxes.forEach(checkbox => {
    checkbox.checked = isChecked;
  });
  
  if (headerCheckbox && selectAllCheckbox) {
    headerCheckbox.checked = isChecked;
    selectAllCheckbox.checked = isChecked;
  }
}

function getSelectedProjects(groupIdx) {
  return Array.from(
    document.querySelectorAll(`.project-checkbox[data-group="${groupIdx}"]:checked`)
  );
}

async function loadSelectedPipelines(groupIdx) {
  const selected = getSelectedProjects(groupIdx);

  if (!selected.length) {
    alert("Выберите хотя бы один проект");
    return;
  }

  await Promise.all(
    selected.map(cb => {
      const projIdx = cb.dataset.project;
      const projectId = cb.dataset.projectid;
      return loadPipeline(groupIdx, projIdx, projectId);
    })
  );
}

function clearAllPipelines() {
  const allJobs = document.querySelectorAll('[id^="jobs-"]');
  allJobs.forEach(tbody => tbody.innerHTML = '');
}

async function openPipeline(groupIdx, projIdx, projectId) {
  const select = document.getElementById(`ref-${groupIdx}-${projIdx}`);
  const ref = select.value;

  const resp = await fetch(
    `/pipeline-url?project_id=${projectId}&ref=${encodeURIComponent(ref)}`
  );
  const data = await resp.json();
  if (data.url) {
    window.open(data.url, '_blank');
  } else {
    alert("Не удалось получить URL пайплайна");
  }
}

function openTab(tabId) {
  document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
  document.getElementById(tabId).classList.add('active');
  event.target.classList.add('active');
}

// Массовый выбор ветки в тегах
function applyTagsGroupRef(groupIdx) {
  const groupSelect = document.getElementById("tags-group-ref-" + groupIdx);
  const selectedRef = groupSelect.value;
  if (!selectedRef) return;

  const projectSelects = document.querySelectorAll(
    `[id^="tags-ref-${groupIdx}-"]`
  );
  projectSelects.forEach((sel) => {
    if ([...sel.options].some((opt) => opt.value === selectedRef)) {
      sel.value = selectedRef;
    }
  });
}

// Переключение всех чекбоксов в группе тегов
function toggleAllTagsProjects(groupIdx) {
  const headerCheckbox = document.getElementById(`tags-header-checkbox-${groupIdx}`);
  const projectCheckboxes = document.querySelectorAll(`.tags-project-checkbox[data-group="${groupIdx}"]`);
  
  const isChecked = headerCheckbox.checked;
    
  projectCheckboxes.forEach(checkbox => {
    checkbox.checked = isChecked;
  });
}

// Получить выбранные проекты в тегах
function getSelectedTagsProjects(groupIdx) {
  return Array.from(
    document.querySelectorAll(`.tags-project-checkbox[data-group="${groupIdx}"]:checked`)
  );
}

// Загрузка тегов для выбранных проектов
async function loadSelectedTags(groupIdx) {
  clearAllTags(groupIdx)
  const selected = getSelectedTagsProjects(groupIdx);

  if (!selected.length) {
    alert("Выберите хотя бы один проект");
    return;
  }

  // Показываем индикатор загрузки
  selected.forEach(cb => {
    const projIdx = cb.dataset.project;
    const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
    tagsDiv.innerHTML = '<em>Загрузка тегов...</em>';
  });

  await Promise.all(
    selected.map(async cb => {
      const projIdx = cb.dataset.project;
      const projectId = cb.dataset.projectid;
      await loadProjectTags(groupIdx, projIdx, projectId);
    })
  );
}

// Очистка всех тегов
function clearAllTags(groupIdx) {
  document.querySelectorAll(`[id^="tags-list-${groupIdx}-"]`).forEach(div => {
    div.innerHTML = "";
  });
}

// Загрузка тегов для конкретного проекта
async function loadProjectTags(groupIdx, projIdx, projectId) {
  const select = document.getElementById(`tags-ref-${groupIdx}-${projIdx}`);
  const ref = select.value;

  try {
    const resp = await fetch(`/tags?project_id=${projectId}&ref=${encodeURIComponent(ref)}`);
    if (!resp.ok) throw new Error("HTTP error " + resp.status);

    const data = await resp.json();
    const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
    
    if (data.error) {
      tagsDiv.innerHTML = `<span style="color: red;">Ошибка: ${data.error}</span>`;
      return;
    }

    // Отображаем список тегов с прокруткой
    if (data.tags && data.tags.length > 0) {
      let html = `<div style="font-weight: bold; margin-bottom: 5px;">
              <span class="tags-count">Тегов: ${data.tags.length}</span>
              <button type="button" 
                      onclick="deleteSelectedTagsInProject('${groupIdx}', '${projIdx}', '${projectId}')" 
                      class="cancel-btn" 
                      style="margin-left: 10px; padding: 2px 6px; font-size: 11px;">
                  Удалить выбранные
              </button>
              <label style="margin-left: 10px; font-weight: normal; font-size: 12px;">
                  <input type="checkbox" id="select-all-${groupIdx}-${projIdx}" 
                         onchange="toggleAllTags('${groupIdx}', '${projIdx}')">
                  Выбрать все
              </label>
            </div>`;
      html += '<ul style="margin: 0; padding-left: 15px; max-height: 200px; overflow-y: auto; border: 1px solid #ddd; border-radius: 4px; padding: 8px;">';
      
      data.tags.forEach(tag => {
        html += `<li style="display: flex; justify-content: space-between; align-items: center; padding: 3px 0; border-bottom: 1px solid #eee;">
                  <span style="display: flex; align-items: center; flex: 1;">
                    <input type="checkbox" class="tag-checkbox" 
                           id="tag-${groupIdx}-${projIdx}-${tag.name}" 
                           data-tag-name="${tag.name}"
                           style="margin-right: 8px;">
                    <span>${tag.name}</span>
                  </span>
                  <button onclick="deleteTag(${groupIdx}, ${projIdx}, ${projectId}, '${tag.name}')" 
                          class="cancel-btn" 
                          style="margin-left: 10px; padding: 2px 6px; font-size: 11px;">
                    Удалить
                  </button>
                </li>`;
            });
      html += '</ul>';
      tagsDiv.innerHTML = html;
    } else {
      tagsDiv.innerHTML = '<em>Тегов нет</em>';
    }
  } catch (err) {
    const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
    tagsDiv.innerHTML = `<span style="color: red;">Ошибка: ${err.message}</span>`;
  }
}

function updateTagsCount(groupIdx, projIdx) {
  const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
  if (!tagsDiv) return;

  const countEl = tagsDiv.querySelector(".tags-count");
  if (!countEl) return;

  const list = tagsDiv.querySelector("ul");
  const count = list ? list.querySelectorAll("li").length : 0;

  countEl.textContent = `Тегов: ${count}`;
}

// Удаление тега
async function deleteTag(groupIdx, projIdx, projectId, tagName, skipConfirm = false) {
  if (!skipConfirm && !confirm(`Удалить тег "${tagName}"?`)) return;

  try {
    const resp = await fetch(`/tags/delete?project_id=${projectId}&tag_name=${encodeURIComponent(tagName)}`, {
      method: 'DELETE'
    });

    if (!resp.ok) throw new Error("HTTP error " + resp.status);

    const data = await resp.json();
    if (data.error) {
      alert("Ошибка: " + data.error);
    } else {
      // Перезагружаем теги
      await loadProjectTags(groupIdx, projIdx, projectId);
      await updatePipelineRefs(groupIdx, projIdx, projectId);
      updateTagsCount(groupIdx, projIdx);
    }
  } catch (err) {
    alert("Ошибка: " + err.message);
  }
}

// Создание тега для выбранных проектов
async function createTagsForSelected(groupIdx) {
  const selected = getSelectedTagsProjects(groupIdx);

  if (!selected.length) {
    alert("Выберите хотя бы один проект");
    return;
  }

  const tagName = prompt("Введите имя тега:");
  if (tagName === null) {
    return;
  }
  if (!tagName.trim()) {
    alert("Имя тега не может быть пустым");
    return;
  }

  // Показываем индикатор создания
  selected.forEach(cb => {
    const projIdx = cb.dataset.project;
    const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
    tagsDiv.innerHTML = '<em>Создание тега...</em>';
  });

  await Promise.all(
    selected.map(async cb => {
      const projIdx = cb.dataset.project;
      const projectId = cb.dataset.projectid;
      await createTag(groupIdx, projIdx, projectId, tagName.trim());
    })
  );
}

// Создание тега для конкретного проекта
async function createTag(groupIdx, projIdx, projectId, tagName) {
  try {
    // Используем выбранный бранч
    const select = document.getElementById(`tags-ref-${groupIdx}-${projIdx}`);
    const refName = select.value;

    const resp = await fetch('/tags/create', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        project_id: projectId,
        tag_name: tagName,
        ref: refName
      })
    });

    if (!resp.ok) throw new Error("HTTP error " + resp.status);

    const data = await resp.json();
    const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
    
    if (data.error) {
      tagsDiv.innerHTML = `<span style="color: red;">Ошибка: ${data.error}</span>`;
    } else {
      tagsDiv.innerHTML = '<span style="color: green;">Тег создан успешно!</span>';
      setTimeout(() => {
          loadProjectTags(groupIdx, projIdx, projectId);
          updatePipelineRefs(groupIdx, projIdx, projectId);
        }, 1000);
    }
  } catch (err) {
    const tagsDiv = document.getElementById(`tags-list-${groupIdx}-${projIdx}`);
    tagsDiv.innerHTML = `<span style="color: red;">Ошибка: ${err.message}</span>`;
  }
}

// Обновляет список рефов в селекторе пайплайнов
async function updatePipelineRefs(groupIdx, projIdx, projectId) {
    try {
        const refSelect = document.getElementById(`ref-${groupIdx}-${projIdx}`);
        if (!refSelect) return;
        
        const currentBranches = Array.from(refSelect.options)
            .map(opt => opt.value)
            .filter(val => val && val !== '—' && !val.startsWith('tag:'));
        
        const currentValue = refSelect.value;
        
        const resp = await fetch(`/tags?project_id=${projectId}&ref=main`);
        if (!resp.ok) return;
        
        const data = await resp.json();
        const tags = data.tags || [];
        
        refSelect.innerHTML = '';
        
        currentBranches.forEach(branch => {
            const option = document.createElement('option');
            option.value = branch;
            option.textContent = branch;
            refSelect.appendChild(option);
        });
        
        tags.forEach(tag => {
            const tagValue = `tag:${tag.name}`;
            const option = document.createElement('option');
            option.value = tagValue;
            option.textContent = tagValue;
            refSelect.appendChild(option);
        });      

        if (Array.from(refSelect.options).some(opt => opt.value === currentValue)) {
            refSelect.value = currentValue;
        } else if (refSelect.options.length > 0) {
            refSelect.selectedIndex = 0;
        }
        
        // Обновляем групповой селект рефов
        updateGroupRefsSelect(groupIdx);
        
    } catch (err) {
        console.error('Ошибка обновления рефов:', err);
    }
}

// Обновляет групповой селект "Массовый выбор"
function updateGroupRefsSelect(groupIdx) {
    const groupSelect = document.getElementById(`group-ref-${groupIdx}`);
    if (!groupSelect) return;

    const allRefs = new Set();
    
    document.querySelectorAll(`[id^="ref-${groupIdx}-"]`).forEach(select => {
        Array.from(select.options).forEach(option => {
            if (option.value && option.value !== '—') {
                allRefs.add(option.value);
            }
        });
    });

    const currentValue = groupSelect.value;

    groupSelect.innerHTML = '<option value="">— Массовый выбор —</option>';
    Array.from(allRefs).sort().forEach(ref => {
        const option = document.createElement('option');
        option.value = ref;
        option.textContent = ref;
        groupSelect.appendChild(option);
    });

    if (allRefs.has(currentValue)) {
        groupSelect.value = currentValue;
    }
}

function toggleAllTags(groupIdx, projIdx) {
    const selectAllCheckbox = document.getElementById(`select-all-${groupIdx}-${projIdx}`);
    const tagCheckboxes = document.querySelectorAll(`.tag-checkbox[id^="tag-${groupIdx}-${projIdx}-"]`);
    
    const isChecked = selectAllCheckbox.checked;
    
    tagCheckboxes.forEach(checkbox => {
        checkbox.checked = isChecked;
    });
}

async function deleteSelectedTagsInProject(groupIdx, projIdx, projectId) {
  const checkboxes = document.querySelectorAll(
    `#tags-list-${groupIdx}-${projIdx} .tag-checkbox:checked`
  );

  if (!checkboxes.length) {
    alert("Выберите хотя бы один тег для удаления");
    return;
  }

  if (!confirm(`Удалить выбранные теги (${checkboxes.length} шт.)?`)) return;

  await Promise.all(
    Array.from(checkboxes).map(cb =>
      deleteTag(groupIdx, projIdx, projectId, cb.dataset.tagName, true)
    )
  );
}
