(() => {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Moscow';
  document.querySelectorAll('[data-timezone-label]').forEach((node) => {
    node.textContent = timezone;
  });
  document.querySelectorAll('[data-timezone-input]').forEach((node) => {
    node.value = timezone;
  });

  const dateFormatter = new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: 'long',
    weekday: 'long',
  });
  const timeFormatter = new Intl.DateTimeFormat('ru-RU', {
    hour: '2-digit',
    minute: '2-digit',
  });

  const slotInput = document.querySelector('[data-slot-input]');
  const selectedSlot = document.querySelector('[data-selected-slot]');
  const slotOptions = Array.from(document.querySelectorAll('[data-slot-id]'));

  slotOptions.forEach((slot) => {
    const start = new Date(slot.dataset.start);
    const end = new Date(slot.dataset.end);
    const timeNode = slot.querySelector('[data-slot-time]');
    const label = `${dateFormatter.format(start)}, ${timeFormatter.format(start)}-${timeFormatter.format(end)}`;
    const isDisabled = slot.disabled;

    if (timeNode) {
      timeNode.textContent = `${timeFormatter.format(start)}-${timeFormatter.format(end)}`;
    }
    slot.dataset.label = label;

    slot.addEventListener('click', () => {
      if (isDisabled) {
        return;
      }
      slotOptions.forEach((option) => option.classList.remove('is-selected'));
      slot.classList.add('is-selected');
      if (slotInput) {
        slotInput.value = slot.dataset.slotId;
      }
      if (selectedSlot) {
        selectedSlot.textContent = `Выбрано: ${label}`;
      }
    });
  });

  if (slotInput && slotInput.value) {
    const current = slotOptions.find((slot) => slot.dataset.slotId === slotInput.value && !slot.disabled);
    if (current) {
      current.click();
    }
  }

  document.querySelectorAll('[data-booking-form]').forEach((form) => {
    form.addEventListener('submit', (event) => {
      if (!slotInput || slotInput.value) {
        return;
      }
      event.preventDefault();
      if (selectedSlot) {
        selectedSlot.textContent = 'Выберите дату и время в календаре';
        selectedSlot.focus?.();
      }
    });
  });

  const parseAdminISODate = (value) => {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec((value || '').trim());
    if (!match) {
      return null;
    }
    return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  };

  const formatAdminISODate = (date) => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  };

  const parseTimeParts = (value) => {
    const match = /^(\d{2}):(\d{2})$/.exec((value || '').trim());
    if (!match) {
      return null;
    }
    return { hour: match[1], minute: match[2] };
  };

  const timeToMinutes = (value) => {
    const parts = parseTimeParts(value);
    if (!parts) {
      return Number.NaN;
    }
    return Number(parts.hour) * 60 + Number(parts.minute);
  };

  const minutesToAdminTime = (value) => {
    const hour = Math.floor(value / 60);
    const minute = value % 60;
    return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
  };

  const adminDateLabelFormatter = new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  });

  const adminMonthLabelFormatter = new Intl.DateTimeFormat('ru-RU', {
    month: 'long',
    year: 'numeric',
  });

  const capitalize = (value) => {
    if (!value) {
      return value;
    }
    return value.charAt(0).toUpperCase() + value.slice(1);
  };

  const announceAdminPopoverOpen = (target) => {
    document.dispatchEvent(new CustomEvent('admin-popover-open', {
      detail: { target },
    }));
  };

  const isAdminPopoverOpen = (popover) => popover.classList.contains('is-open');

  const eventIncludesNode = (event, node) => {
    if (!event || !node) {
      return false;
    }
    if (typeof event.composedPath === 'function') {
      return event.composedPath().includes(node);
    }
    return node.contains(event.target);
  };

  const openAdminPopover = (popover) => {
    window.clearTimeout(popover.__adminHideTimer);
    popover.hidden = false;
    popover.setAttribute('aria-hidden', 'false');
    void popover.offsetWidth;
    popover.classList.add('is-open');
  };

  const closeAdminPopover = (popover) => {
    popover.classList.remove('is-open');
    popover.setAttribute('aria-hidden', 'true');
    window.clearTimeout(popover.__adminHideTimer);
    popover.__adminHideTimer = window.setTimeout(() => {
      if (!popover.classList.contains('is-open')) {
        popover.hidden = true;
      }
    }, 180);
  };

  const isSameCalendarDay = (left, right) => (
    Boolean(left)
    && Boolean(right)
    && left.getFullYear() === right.getFullYear()
    && left.getMonth() === right.getMonth()
    && left.getDate() === right.getDate()
  );

  document.querySelectorAll('[data-admin-date-picker]').forEach((picker) => {
    const hiddenInput = picker.querySelector('[data-admin-date-input]');
    const trigger = picker.querySelector('[data-admin-date-trigger]');
    const label = picker.querySelector('[data-admin-date-label]');
    const popover = picker.querySelector('[data-admin-date-popover]');
    const title = picker.querySelector('[data-admin-date-title]');
    const grid = picker.querySelector('[data-admin-date-grid]');
    const prevButton = picker.querySelector('[data-admin-date-prev]');
    const nextButton = picker.querySelector('[data-admin-date-next]');
    const clearButton = picker.querySelector('[data-admin-date-clear]');
    const todayButton = picker.querySelector('[data-admin-date-today]');

    if (!hiddenInput || !trigger || !label || !popover || !title || !grid) {
      return;
    }

    const today = new Date();
    let selectedDate = parseAdminISODate(hiddenInput.value);
    let visibleMonth = selectedDate
      ? new Date(selectedDate.getFullYear(), selectedDate.getMonth(), 1)
      : new Date(today.getFullYear(), today.getMonth(), 1);

    const closePopover = () => {
      closeAdminPopover(popover);
      trigger.setAttribute('aria-expanded', 'false');
    };

    const openPopover = () => {
      announceAdminPopoverOpen(picker);
      openAdminPopover(popover);
      trigger.setAttribute('aria-expanded', 'true');
    };

    const syncLabel = () => {
      const emptyLabel = label.dataset.emptyLabel || 'Выберите дату';
      label.textContent = selectedDate ? adminDateLabelFormatter.format(selectedDate) : emptyLabel;
      trigger.classList.toggle('is-filled', Boolean(selectedDate));
    };

    const renderCalendar = () => {
      title.textContent = capitalize(adminMonthLabelFormatter.format(visibleMonth));
      grid.innerHTML = '';

      const firstOfMonth = new Date(visibleMonth.getFullYear(), visibleMonth.getMonth(), 1);
      const firstWeekday = (firstOfMonth.getDay() + 6) % 7;
      const gridStart = new Date(firstOfMonth);
      gridStart.setDate(firstOfMonth.getDate() - firstWeekday);

      for (let index = 0; index < 42; index += 1) {
        const currentDate = new Date(gridStart);
        currentDate.setDate(gridStart.getDate() + index);

        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'admin-date-day';
        button.textContent = String(currentDate.getDate());

        if (currentDate.getMonth() !== visibleMonth.getMonth()) {
          button.classList.add('is-outside');
        }
        if (isSameCalendarDay(currentDate, today)) {
          button.classList.add('is-today');
        }
        if (isSameCalendarDay(currentDate, selectedDate)) {
          button.classList.add('is-selected');
        }

        button.addEventListener('click', () => {
          selectedDate = currentDate;
          hiddenInput.value = formatAdminISODate(currentDate);
          hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
          visibleMonth = new Date(currentDate.getFullYear(), currentDate.getMonth(), 1);
          syncLabel();
          renderCalendar();
          closePopover();
        });

        grid.appendChild(button);
      }
    };

    trigger.addEventListener('click', () => {
      if (!isAdminPopoverOpen(popover)) {
        openPopover();
      } else {
        closePopover();
      }
    });

    prevButton?.addEventListener('click', () => {
      visibleMonth = new Date(visibleMonth.getFullYear(), visibleMonth.getMonth() - 1, 1);
      renderCalendar();
    });

    nextButton?.addEventListener('click', () => {
      visibleMonth = new Date(visibleMonth.getFullYear(), visibleMonth.getMonth() + 1, 1);
      renderCalendar();
    });

    clearButton?.addEventListener('click', () => {
      selectedDate = null;
      hiddenInput.value = '';
      hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
      syncLabel();
      renderCalendar();
      closePopover();
    });

    todayButton?.addEventListener('click', () => {
      selectedDate = today;
      hiddenInput.value = formatAdminISODate(today);
      hiddenInput.dispatchEvent(new Event('change', { bubbles: true }));
      visibleMonth = new Date(today.getFullYear(), today.getMonth(), 1);
      syncLabel();
      renderCalendar();
      closePopover();
    });

    document.addEventListener('click', (event) => {
      if (!eventIncludesNode(event, picker)) {
        closePopover();
      }
    });

    document.addEventListener('admin-popover-open', (event) => {
      if (event.detail?.target !== picker) {
        closePopover();
      }
    });

    picker.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        closePopover();
      }
    });

    popover.setAttribute('aria-hidden', 'true');
    syncLabel();
    renderCalendar();
  });

  document.querySelectorAll('[data-admin-date-form]').forEach((form) => {
    const dateInput = form.querySelector('[data-admin-date-input]');
    const timePicker = form.querySelector('[data-admin-time-picker]');
    const timePopover = form.querySelector('[data-admin-time-popover]');
    const timeTitle = form.querySelector('[data-admin-time-title]');
    const hoursList = form.querySelector('[data-admin-time-hours]');
    const minutesList = form.querySelector('[data-admin-time-minutes]');
    const applyTimeButton = form.querySelector('[data-admin-time-apply]');
    const clearTimeButton = form.querySelector('[data-admin-time-clear]');
    const selectedList = form.querySelector('[data-date-slot-selected]');
    const hiddenTimes = form.querySelector('[data-date-slot-times]');
    const currentSlot = form.querySelector('[data-date-slot-current]');
    const addButton = form.querySelector('[data-date-slot-add]');
    const clearSlotsButton = form.querySelector('[data-date-slot-clear]');
    const triggerButtons = {
      start: form.querySelector('[data-admin-time-trigger="start"]'),
      end: form.querySelector('[data-admin-time-trigger="end"]'),
    };
    const valueNodes = {
      start: form.querySelector('[data-admin-time-value="start"]'),
      end: form.querySelector('[data-admin-time-value="end"]'),
    };

    if (!timePicker || !timePopover || !timeTitle || !hoursList || !minutesList || !selectedList || !hiddenTimes || !currentSlot || !addButton) {
      return;
    }

    let loadToken = 0;
    let activeTimeField = 'start';
    let draftTime = '';
    let currentRange = {
      start: '',
      end: '',
    };

    const normalizeRangeValue = (value) => {
      const parts = String(value || '').split('-');
      if (parts.length !== 2) {
        return null;
      }
      const start = (parts[0] || '').trim();
      const end = (parts[1] || '').trim();
      if (!parseTimeParts(start) || !parseTimeParts(end)) {
        return null;
      }
      return { start, end };
    };

    const rangeKey = (range) => `${range.start}-${range.end}`;

    const normalizeSelectedRanges = (values) => {
      const unique = new Map();
      values.forEach((value) => {
        const range = typeof value === 'string' ? normalizeRangeValue(value) : normalizeRangeValue(rangeKey(value));
        if (!range) {
          return;
        }
        unique.set(rangeKey(range), range);
      });
      return Array.from(unique.values()).sort((left, right) => {
        const diff = timeToMinutes(left.start) - timeToMinutes(right.start);
        if (diff !== 0) {
          return diff;
        }
        return timeToMinutes(left.end) - timeToMinutes(right.end);
      });
    };

    const rangesOverlap = (left, right) => {
      const leftStart = timeToMinutes(left.start);
      const leftEnd = timeToMinutes(left.end);
      const rightStart = timeToMinutes(right.start);
      const rightEnd = timeToMinutes(right.end);
      return leftStart < rightEnd && rightStart < leftEnd;
    };

    const replaceOverlappingRanges = (values, nextRange) => normalizeSelectedRanges([
      ...values.filter((value) => !rangesOverlap(value, nextRange)),
      nextRange,
    ]);

    const availableTimesForField = (field) => {
      const dayStart = 9 * 60;
      const dayEnd = 22 * 60 + 30;
      let minMinutes = field === 'start' ? dayStart : dayStart + 1;
      let maxMinutes = field === 'start' ? dayEnd - 1 : dayEnd;

      if (field === 'start' && currentRange.end) {
        maxMinutes = Math.min(maxMinutes, timeToMinutes(currentRange.end) - 1);
      }
      if (field === 'end' && currentRange.start) {
        minMinutes = Math.max(minMinutes, timeToMinutes(currentRange.start) + 1);
      }
      if (!Number.isFinite(minMinutes) || !Number.isFinite(maxMinutes) || minMinutes > maxMinutes) {
        return [];
      }

      const values = [];
      for (let value = minMinutes; value <= maxMinutes; value += 1) {
        values.push(minutesToAdminTime(value));
      }
      return values;
    };

    const validateCurrentRange = (start, end) => {
      if (!parseTimeParts(start) || !parseTimeParts(end)) {
        return 'Выберите корректное время';
      }

      const startMinutes = timeToMinutes(start);
      const endMinutes = timeToMinutes(end);
      if (startMinutes < 9 * 60) {
        return 'Время слота должно начинаться не раньше 09:00';
      }
      if (endMinutes > 22 * 60 + 30) {
        return 'Время слота должно заканчиваться не позже 22:30';
      }
      if (endMinutes <= startMinutes) {
        return 'Конец диапазона должен быть позже начала';
      }
      return '';
    };

    let selectedRanges = normalizeSelectedRanges(hiddenTimes.value.split('\n'));

    const syncTimeTrigger = (field) => {
      const valueNode = valueNodes[field];
      const trigger = triggerButtons[field];
      if (!valueNode || !trigger) {
        return;
      }
      const emptyLabel = valueNode.dataset.emptyLabel || 'Выберите время';
      valueNode.textContent = currentRange[field] || emptyLabel;
      trigger.classList.toggle('is-filled', Boolean(currentRange[field]));
    };

    const closeTimePopover = () => {
      closeAdminPopover(timePopover);
      Object.values(triggerButtons).forEach((button) => {
        button?.setAttribute('aria-expanded', 'false');
      });
    };

    const getValidTimes = (field) => availableTimesForField(field);

    const ensureDraftTime = () => {
      const validTimes = getValidTimes(activeTimeField);
      if (!validTimes.length) {
        draftTime = '';
        return;
      }
      if (!validTimes.includes(draftTime)) {
        draftTime = currentRange[activeTimeField] && validTimes.includes(currentRange[activeTimeField])
          ? currentRange[activeTimeField]
          : validTimes[0];
      }
    };

    const renderTimePopover = () => {
      ensureDraftTime();
      const validTimes = getValidTimes(activeTimeField);
      const parts = parseTimeParts(draftTime) || parseTimeParts(validTimes[0]);
      if (!parts || !validTimes.length) {
        timeTitle.textContent = activeTimeField === 'start' ? 'Выберите время начала' : 'Выберите время окончания';
        hoursList.innerHTML = '';
        minutesList.innerHTML = '';
        return;
      }

      const hours = Array.from(new Set(validTimes.map((value) => value.slice(0, 2))));
      let selectedHour = parts.hour;
      if (!hours.includes(selectedHour)) {
        selectedHour = hours[0];
      }

      const minutes = validTimes
        .filter((value) => value.startsWith(`${selectedHour}:`))
        .map((value) => value.slice(3));

      let selectedMinute = parts.minute;
      if (!minutes.includes(selectedMinute)) {
        selectedMinute = minutes[0];
      }

      draftTime = `${selectedHour}:${selectedMinute}`;
      timeTitle.textContent = activeTimeField === 'start' ? 'Выберите время начала' : 'Выберите время окончания';
      hoursList.innerHTML = '';
      minutesList.innerHTML = '';

      hours.forEach((hour) => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'admin-time-option';
        button.textContent = hour;
        if (hour === selectedHour) {
          button.classList.add('is-selected');
        }
        button.addEventListener('click', () => {
          const nextMinutes = validTimes
            .filter((value) => value.startsWith(`${hour}:`))
            .map((value) => value.slice(3));
          draftTime = `${hour}:${nextMinutes.includes(selectedMinute) ? selectedMinute : nextMinutes[0]}`;
          renderTimePopover();
        });
        hoursList.appendChild(button);
      });

      minutes.forEach((minute) => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'admin-time-option';
        button.textContent = minute;
        if (minute === selectedMinute) {
          button.classList.add('is-selected');
        }
        button.addEventListener('click', () => {
          draftTime = `${selectedHour}:${minute}`;
          applyTimeValue(activeTimeField, draftTime);
        });
        minutesList.appendChild(button);
      });
    };

    const applyTimeValue = (field, value, { closePopoverOnApply = true } = {}) => {
      const validTimes = getValidTimes(field);
      if (!value || !validTimes.includes(value)) {
        syncCurrentSlot('Выберите корректное время');
        if (closePopoverOnApply) {
          closeTimePopover();
        }
        return false;
      }

      currentRange = {
        ...currentRange,
        [field]: value,
      };
      draftTime = value;
      syncTimeTrigger('start');
      syncTimeTrigger('end');
      syncCurrentSlot();

      if (closePopoverOnApply) {
        closeTimePopover();
      }

      return true;
    };

    const applyDraftTime = ({ closePopoverOnApply = true } = {}) => applyTimeValue(activeTimeField, draftTime, { closePopoverOnApply });

    const openTimePopover = (field) => {
      activeTimeField = field;
      draftTime = currentRange[field] || getValidTimes(field)[0] || '';
      renderTimePopover();
      announceAdminPopoverOpen(timePicker);
      openAdminPopover(timePopover);
      Object.entries(triggerButtons).forEach(([name, button]) => {
        button?.setAttribute('aria-expanded', name === field ? 'true' : 'false');
      });
    };

    const syncCurrentSlot = (message = '') => {
      if (message) {
        currentSlot.textContent = message;
        return;
      }
      if (currentRange.start && currentRange.end) {
        currentSlot.textContent = `Текущий диапазон: ${currentRange.start}-${currentRange.end}`;
        return;
      }
      currentSlot.textContent = 'Сначала выберите диапазон времени';
    };

    const renderSelected = () => {
      hiddenTimes.value = selectedRanges.map((range) => rangeKey(range)).join('\n');
      selectedList.innerHTML = '';

      if (!selectedRanges.length) {
        const empty = document.createElement('p');
        empty.className = 'admin-selected-slots__empty';
        empty.textContent = 'Пока ничего не добавлено.';
        selectedList.appendChild(empty);
        return;
      }

      selectedRanges.forEach((range) => {
        const chip = document.createElement('div');
        chip.className = 'admin-selected-slot-chip';

        const text = document.createElement('span');
        text.textContent = rangeKey(range);

        const remove = document.createElement('button');
        remove.type = 'button';
        remove.setAttribute('aria-label', 'Удалить слот');
        remove.textContent = '×';
        remove.addEventListener('click', () => {
          selectedRanges = selectedRanges.filter((value) => rangeKey(value) !== rangeKey(range));
          renderSelected();
        });

        chip.append(text, remove);
        selectedList.appendChild(chip);
      });
    };

    const resetCurrentRange = () => {
      currentRange = { start: '', end: '' };
      draftTime = '';
      syncTimeTrigger('start');
      syncTimeTrigger('end');
    };

    const loadDateSlots = async (date) => {
      resetCurrentRange();

      if (!dateInput) {
        renderSelected();
        syncCurrentSlot();
        return;
      }

      if (!date) {
        selectedRanges = [];
        renderSelected();
        syncCurrentSlot('Сначала выберите диапазон времени');
        return;
      }

      const token = ++loadToken;
      syncCurrentSlot('Загружаю слоты на эту дату...');

      try {
        const response = await fetch(`/administrator/slots/day?date=${encodeURIComponent(date)}`, {
          headers: {
            Accept: 'application/json',
          },
        });
        if (!response.ok) {
          throw new Error('load failed');
        }

        const payload = await response.json();
        if (token !== loadToken) {
          return;
        }

        selectedRanges = normalizeSelectedRanges(Array.isArray(payload.ranges) ? payload.ranges : []);
        renderSelected();
        if (selectedRanges.length > 0) {
          syncCurrentSlot('Для этой даты уже сохранено отдельное расписание. Можно изменить интервалы и сохранить заново.');
        } else {
          syncCurrentSlot('На эту дату пока нет отдельного расписания. Добавьте нужные интервалы.');
        }
      } catch (error) {
        if (token !== loadToken) {
          return;
        }
        selectedRanges = [];
        renderSelected();
        syncCurrentSlot('Не удалось загрузить слоты на эту дату');
      }
    };

    triggerButtons.start?.addEventListener('click', () => {
      if (isAdminPopoverOpen(timePopover) && activeTimeField === 'start') {
        closeTimePopover();
        return;
      }
      openTimePopover('start');
    });

    triggerButtons.end?.addEventListener('click', () => {
      if (isAdminPopoverOpen(timePopover) && activeTimeField === 'end') {
        closeTimePopover();
        return;
      }
      openTimePopover('end');
    });

    applyTimeButton?.addEventListener('click', () => {
      applyDraftTime();
    });

    clearTimeButton?.addEventListener('click', () => {
      currentRange = {
        ...currentRange,
        [activeTimeField]: '',
      };
      draftTime = '';
      syncTimeTrigger('start');
      syncTimeTrigger('end');
      syncCurrentSlot();
      closeTimePopover();
    });

    addButton.addEventListener('click', () => {
      const validationMessage = validateCurrentRange(currentRange.start, currentRange.end);
      if (validationMessage) {
        syncCurrentSlot(validationMessage);
        return;
      }

      const nextRange = { ...currentRange };
      if (selectedRanges.some((range) => rangeKey(range) === rangeKey(nextRange))) {
        syncCurrentSlot(`Этот диапазон уже добавлен: ${rangeKey(nextRange)}`);
        return;
      }

      const removedRanges = selectedRanges.filter((range) => rangesOverlap(range, nextRange));
      selectedRanges = replaceOverlappingRanges(selectedRanges, nextRange);
      renderSelected();
      resetCurrentRange();
      closeTimePopover();
      if (removedRanges.length > 0) {
        syncCurrentSlot(`Добавлено: ${rangeKey(nextRange)}. Пересекающиеся диапазоны на эту дату обновлены.`);
        return;
      }
      syncCurrentSlot(`Добавлено: ${rangeKey(nextRange)}`);
    });

    clearSlotsButton?.addEventListener('click', () => {
      selectedRanges = [];
      renderSelected();
      syncCurrentSlot('Список диапазонов на эту дату очищен');
    });

    dateInput?.addEventListener('change', () => {
      loadDateSlots(dateInput.value);
    });

    form.addEventListener('submit', (event) => {
      if (!dateInput?.value) {
        event.preventDefault();
        syncCurrentSlot('Сначала выберите дату');
        return;
      }
      if (!selectedRanges.length) {
        event.preventDefault();
        syncCurrentSlot('Добавьте хотя бы один диапазон на выбранную дату');
      }
    });

    document.addEventListener('click', (event) => {
      if (!eventIncludesNode(event, timePicker)) {
        closeTimePopover();
      }
    });

    document.addEventListener('admin-popover-open', (event) => {
      if (event.detail?.target !== timePicker) {
        closeTimePopover();
      }
    });

    timePicker.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        closeTimePopover();
      }
    });

    timePopover.setAttribute('aria-hidden', 'true');
    resetCurrentRange();
    renderSelected();
    syncCurrentSlot();

    if (dateInput?.value && !hiddenTimes.value.trim()) {
      loadDateSlots(dateInput.value);
    }
  });

  document.querySelectorAll('[data-weekday-card]').forEach((card) => {
    const toggle = card.querySelector('[data-weekday-toggle]');
    const toggleLabel = card.querySelector('[data-weekday-toggle-label]');
    const statusLabel = card.querySelector('[data-weekday-status]');

    if (!toggle || !toggleLabel || !statusLabel) {
      return;
    }

    const syncWeekdayCard = () => {
      const enabled = toggle.checked;
      card.classList.toggle('is-enabled', enabled);
      card.classList.toggle('is-disabled', !enabled);
      toggleLabel.textContent = enabled ? 'Включено' : 'Выключено';
      statusLabel.textContent = enabled ? 'Рабочий день' : 'День выключен';
    };

    toggle.addEventListener('change', syncWeekdayCard);
    syncWeekdayCard();
  });
})();
