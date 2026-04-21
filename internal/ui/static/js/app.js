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

  const adminDateLabelFormatter = new Intl.DateTimeFormat('ru-RU', {
    day: 'numeric',
    month: 'long',
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

    if (!hiddenInput || !trigger || !label || !popover || !title || !grid) {
      return;
    }

    const today = new Date();
    let selectedDate = parseAdminISODate(hiddenInput.value);
    let visibleMonth = selectedDate
      ? new Date(selectedDate.getFullYear(), selectedDate.getMonth(), 1)
      : new Date(today.getFullYear(), today.getMonth(), 1);

    const closePopover = () => {
      popover.hidden = true;
      trigger.setAttribute('aria-expanded', 'false');
    };

    const openPopover = () => {
      popover.hidden = false;
      trigger.setAttribute('aria-expanded', 'true');
    };

    const syncLabel = () => {
      const emptyLabel = label.dataset.emptyLabel || 'Выберите дату';
      label.textContent = selectedDate ? capitalize(adminDateLabelFormatter.format(selectedDate)) : emptyLabel;
    };

    const renderCalendar = () => {
      title.textContent = capitalize(adminMonthLabelFormatter.format(visibleMonth));
      grid.innerHTML = '';

      const year = visibleMonth.getFullYear();
      const month = visibleMonth.getMonth();
      const firstDay = new Date(year, month, 1);
      const firstWeekday = (firstDay.getDay() + 6) % 7;
      const daysInMonth = new Date(year, month + 1, 0).getDate();

      for (let index = 0; index < firstWeekday; index += 1) {
        const blank = document.createElement('span');
        blank.className = 'admin-date-grid__blank';
        grid.appendChild(blank);
      }

      for (let day = 1; day <= daysInMonth; day += 1) {
        const currentDate = new Date(year, month, day);
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'admin-date-day';
        button.textContent = String(day);

        if (isSameCalendarDay(currentDate, today)) {
          button.classList.add('is-today');
        }
        if (isSameCalendarDay(currentDate, selectedDate)) {
          button.classList.add('is-selected');
        }

        button.addEventListener('click', () => {
          selectedDate = currentDate;
          hiddenInput.value = formatAdminISODate(currentDate);
          visibleMonth = new Date(currentDate.getFullYear(), currentDate.getMonth(), 1);
          syncLabel();
          renderCalendar();
          closePopover();
        });

        grid.appendChild(button);
      }
    };

    trigger.addEventListener('click', () => {
      if (popover.hidden) {
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

    document.addEventListener('click', (event) => {
      if (!picker.contains(event.target)) {
        closePopover();
      }
    });

    picker.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        closePopover();
      }
    });

    syncLabel();
    renderCalendar();
  });

  document.querySelectorAll('[data-admin-date-form]').forEach((form) => {
    const startSelect = form.querySelector('[data-date-slot-start]');
    const endSelect = form.querySelector('[data-date-slot-end]');
    const selectedList = form.querySelector('[data-date-slot-selected]');
    const hiddenTimes = form.querySelector('[data-date-slot-times]');
    const currentSlot = form.querySelector('[data-date-slot-current]');
    const addButton = form.querySelector('[data-date-slot-add]');
    const clearButton = form.querySelector('[data-date-slot-clear]');

    if (!startSelect || !endSelect || !selectedList || !hiddenTimes || !currentSlot || !addButton) {
      return;
    }

    const optionMap = new Map(
      Array.from(startSelect.options)
        .filter((option) => option.value)
        .map((option) => [option.value, {
          value: option.value,
          end: option.dataset.end || '',
          label: option.dataset.label || option.value,
        }]),
    );

    let selectedSlots = hiddenTimes.value
      .split('\n')
      .map(normalizeAdminTime)
      .filter(Boolean)
      .filter((value, index, array) => array.indexOf(value) === index)
      .sort();

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
        const option = optionMap.get(value);
        const chip = document.createElement('div');
        chip.className = 'admin-selected-slot-chip';

        const text = document.createElement('span');
        text.textContent = option?.label || value;

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

    const resetEndSelect = (placeholder) => {
      endSelect.innerHTML = '';
      const option = document.createElement('option');
      option.value = '';
      option.textContent = placeholder;
      endSelect.appendChild(option);
      endSelect.value = '';
    };

    const syncRangeControls = () => {
      const selected = optionMap.get(startSelect.value);
      if (!selected) {
        resetEndSelect('Сначала выберите начало');
        endSelect.disabled = true;
        currentSlot.textContent = 'Сначала выберите начало и конец диапазона';
        return;
      }

      resetEndSelect(selected.end);
      endSelect.disabled = false;
      endSelect.value = selected.end;
      currentSlot.textContent = `Выбран диапазон: ${selected.label}`;
    };

    startSelect.addEventListener('change', syncRangeControls);

    addButton.addEventListener('click', () => {
      const selected = optionMap.get(startSelect.value);
      if (!selected || !endSelect.value || endSelect.value !== selected.end) {
        currentSlot.textContent = 'Выберите корректный диапазон времени';
        return;
      }

      if (selectedSlots.includes(selected.value)) {
        currentSlot.textContent = `Этот диапазон уже добавлен: ${selected.label}`;
        return;
      }

      selectedSlots = [...selectedSlots, selected.value].sort();
      renderSelected();
      currentSlot.textContent = `Добавлено: ${selected.label}`;
      startSelect.value = '';
      syncRangeControls();
    });

    clearButton?.addEventListener('click', () => {
      selectedSlots = [];
      renderSelected();
      startSelect.value = '';
      syncRangeControls();
    });

    renderSelected();
    syncRangeControls();
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
