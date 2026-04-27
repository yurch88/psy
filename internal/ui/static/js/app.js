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

  const normalizeAdminTime = (value) => {
    const normalized = (value || '').trim();
    if (!normalized) {
      return '';
    }
    const [start] = normalized.split('-');
    return (start || '').trim();
  };

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

  const adminSlotDurationMinutes = 55;

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

  const slotEndForStart = (value) => {
    const startMinutes = timeToMinutes(value);
    if (!Number.isFinite(startMinutes)) {
      return '';
    }
    return minutesToAdminTime(startMinutes + adminSlotDurationMinutes);
  };

  const adminRangesOverlap = (firstStart, secondStart) => {
    const firstMinutes = timeToMinutes(firstStart);
    const secondMinutes = timeToMinutes(secondStart);
    if (!Number.isFinite(firstMinutes) || !Number.isFinite(secondMinutes)) {
      return false;
    }

    const firstEnd = firstMinutes + adminSlotDurationMinutes;
    const secondEnd = secondMinutes + adminSlotDurationMinutes;
    return firstMinutes < secondEnd && secondMinutes < firstEnd;
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
    const sourceNodes = Array.from(form.querySelectorAll('[data-slot-value]'));

    if (!timePicker || !timePopover || !timeTitle || !hoursList || !minutesList || !selectedList || !hiddenTimes || !currentSlot || !addButton) {
      return;
    }

    const slotOptions = sourceNodes.map((node) => ({
      value: node.dataset.slotValue || '',
      end: node.dataset.slotEnd || '',
      label: node.dataset.slotLabel || node.dataset.slotValue || '',
    })).filter((option) => option.value && option.end);

    const optionMap = new Map(slotOptions.map((option) => [option.value, option.label]));
    const startToEnd = new Map(slotOptions.map((option) => [option.value, option.end]));
    const endToStart = new Map(slotOptions.map((option) => [option.end, option.value]));
    const validStartTimes = slotOptions.map((option) => option.value);
    const validEndTimes = Array.from(new Set(slotOptions.map((option) => option.end))).sort();
    let loadToken = 0;

    const normalizeSelectedSlots = (values) => values
      .map(normalizeAdminTime)
      .filter(Boolean)
      .filter((value, index, array) => array.indexOf(value) === index)
      .sort((left, right) => timeToMinutes(left) - timeToMinutes(right));

    const slotLabel = (value) => optionMap.get(value) || `${value}-${slotEndForStart(value)}`;

    const replaceOverlappingSlots = (values, priorityValue) => normalizeSelectedSlots([
      ...values.filter((value) => value === priorityValue || !adminRangesOverlap(value, priorityValue)),
      priorityValue,
    ]);

    let selectedSlots = normalizeSelectedSlots(hiddenTimes.value.split('\n'));

    let currentRange = {
      start: '',
      end: '',
    };
    let activeTimeField = 'start';
    let draftTime = '';

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

    const getValidTimes = (field) => (field === 'end' ? validEndTimes : validStartTimes);

    const resolveRangeFromTime = (field, value) => {
      if (!value) {
        return { start: '', end: '' };
      }
      if (field === 'start') {
        return {
          start: value,
          end: startToEnd.get(value) || '',
        };
      }
      return {
        start: endToStart.get(value) || '',
        end: value,
      };
    };

    const ensureDraftTime = () => {
      const validTimes = getValidTimes(activeTimeField);
      if (!validTimes.length) {
        draftTime = '';
        return;
      }
      if (!validTimes.includes(draftTime)) {
        draftTime = validTimes[0];
      }
    };

    const renderTimePopover = () => {
      ensureDraftTime();
      const validTimes = getValidTimes(activeTimeField);
      const parts = parseTimeParts(draftTime) || parseTimeParts(validTimes[0]);
      if (!parts) {
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

    const applyDraftTime = ({ closePopoverOnApply = true } = {}) => {
      return applyTimeValue(activeTimeField, draftTime, { closePopoverOnApply });
    };

    const applyTimeValue = (field, value, { closePopoverOnApply = true } = {}) => {
      if (!value) {
        syncCurrentSlot('Выберите корректное время');
        if (closePopoverOnApply) {
          closeTimePopover();
        }
        return false;
      }

      const nextRange = resolveRangeFromTime(field, value);
      if (!nextRange.start || !nextRange.end) {
        syncCurrentSlot('Выберите корректное время');
        if (closePopoverOnApply) {
          closeTimePopover();
        }
        return false;
      }

      currentRange = nextRange;
      draftTime = value;
      syncTimeTrigger('start');
      syncTimeTrigger('end');
      syncCurrentSlot();

      if (closePopoverOnApply) {
        closeTimePopover();
      }

      return true;
    };

    const openTimePopover = (field) => {
      activeTimeField = field;
      if (field === 'start') {
        draftTime = currentRange.start || validStartTimes[0] || '';
      } else {
        draftTime = currentRange.end || (currentRange.start ? (startToEnd.get(currentRange.start) || '') : '') || validEndTimes[0] || '';
      }
      renderTimePopover();
      announceAdminPopoverOpen(timePicker);
      openAdminPopover(timePopover);
      Object.entries(triggerButtons).forEach(([name, button]) => {
        button?.setAttribute('aria-expanded', name === field ? 'true' : 'false');
      });
    };

    const renderSelected = () => {
      hiddenTimes.value = selectedSlots.join('\n');
      selectedList.innerHTML = '';

      if (!selectedSlots.length) {
        const empty = document.createElement('p');
        empty.className = 'admin-selected-slots__empty';
        empty.textContent = 'Пока ничего не добавлено.';
        selectedList.appendChild(empty);
        return;
      }

      selectedSlots.forEach((value) => {
        const chip = document.createElement('div');
        chip.className = 'admin-selected-slot-chip';

        const text = document.createElement('span');
        text.textContent = slotLabel(value);

        const remove = document.createElement('button');
        remove.type = 'button';
        remove.setAttribute('aria-label', 'Удалить слот');
        remove.textContent = '×';
        remove.addEventListener('click', () => {
          selectedSlots = selectedSlots.filter((slot) => slot !== value);
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
        selectedSlots = [];
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

        selectedSlots = normalizeSelectedSlots(Array.isArray(payload.times) ? payload.times : []);
        renderSelected();
        if (selectedSlots.length > 0) {
          syncCurrentSlot('Слоты на дату загружены. Можно убрать лишние или добавить свои.');
        } else {
          syncCurrentSlot('На эту дату пока нет слотов. Добавьте нужные интервалы.');
        }
      } catch (error) {
        if (token !== loadToken) {
          return;
        }
        selectedSlots = [];
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
      resetCurrentRange();
      syncCurrentSlot();
      closeTimePopover();
    });

    addButton.addEventListener('click', () => {
      const expectedEnd = currentRange.start ? startToEnd.get(currentRange.start) : '';
      if (!currentRange.start || !currentRange.end || currentRange.end !== expectedEnd) {
        syncCurrentSlot('Выберите корректный диапазон времени');
        return;
      }

      if (selectedSlots.includes(currentRange.start)) {
        syncCurrentSlot(`Этот диапазон уже добавлен: ${slotLabel(currentRange.start)}`);
        return;
      }

      const removedSlots = selectedSlots.filter((value) => adminRangesOverlap(value, currentRange.start));
      const addedLabel = slotLabel(currentRange.start);
      selectedSlots = replaceOverlappingSlots(selectedSlots, currentRange.start);
      renderSelected();
      resetCurrentRange();
      closeTimePopover();
      if (removedSlots.length > 0) {
        syncCurrentSlot(`Добавлено: ${addedLabel}. Пересекающиеся слоты на эту дату обновлены.`);
        return;
      }
      syncCurrentSlot(`Добавлено: ${addedLabel}`);
    });

    clearSlotsButton?.addEventListener('click', () => {
      selectedSlots = [];
      renderSelected();
      syncCurrentSlot('Список слотов на эту дату очищен');
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
      if (!selectedSlots.length) {
        event.preventDefault();
        syncCurrentSlot('Добавьте хотя бы один слот на выбранную дату');
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
