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

  document.querySelectorAll('[data-admin-date-form]').forEach((form) => {
    const carousel = form.querySelector('[data-date-slot-carousel]');
    const optionButtons = Array.from(form.querySelectorAll('[data-date-slot-option]'));
    const selectedList = form.querySelector('[data-date-slot-selected]');
    const hiddenTimes = form.querySelector('[data-date-slot-times]');
    const currentSlot = form.querySelector('[data-date-slot-current]');
    const addButton = form.querySelector('[data-date-slot-add]');
    const clearButton = form.querySelector('[data-date-slot-clear]');
    const prevButton = form.querySelector('[data-date-slot-prev]');
    const nextButton = form.querySelector('[data-date-slot-next]');

    if (!carousel || !selectedList || !hiddenTimes || !currentSlot || !addButton) {
      return;
    }

    const optionMap = new Map(optionButtons.map((button) => [button.dataset.dateSlotOption, button.dataset.dateSlotLabel]));
    let activeSlot = '';
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
        const chip = document.createElement('div');
        chip.className = 'admin-selected-slot-chip';

        const text = document.createElement('span');
        text.textContent = optionMap.get(value) || value;

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

    const setActive = (value) => {
      activeSlot = value;
      optionButtons.forEach((button) => {
        button.classList.toggle('is-active', button.dataset.dateSlotOption === value);
      });
      currentSlot.textContent = value
        ? `Выбран диапазон: ${optionMap.get(value) || value}`
        : 'Сначала выберите диапазон времени';
    };

    optionButtons.forEach((button) => {
      button.addEventListener('click', () => {
        setActive(button.dataset.dateSlotOption || '');
      });
    });

    addButton.addEventListener('click', () => {
      if (!activeSlot) {
        currentSlot.textContent = 'Сначала выберите диапазон времени';
        return;
      }
      if (!selectedSlots.includes(activeSlot)) {
        selectedSlots = [...selectedSlots, activeSlot].sort();
        renderSelected();
      }
    });

    clearButton?.addEventListener('click', () => {
      selectedSlots = [];
      renderSelected();
    });

    prevButton?.addEventListener('click', () => {
      carousel.scrollBy({ left: -320, behavior: 'smooth' });
    });

    nextButton?.addEventListener('click', () => {
      carousel.scrollBy({ left: 320, behavior: 'smooth' });
    });

    renderSelected();
    setActive(selectedSlots[0] || '');
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
