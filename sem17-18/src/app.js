let isEditing = false;
let currentEditId = null;

document.addEventListener('DOMContentLoaded', () => {
    initNotes();
    setupEventListeners();
    registerServiceWorker();
});

function initNotes() {
    const notes = JSON.parse(localStorage.getItem('notes') || '[]');
    renderNotes(notes);
}

function setupEventListeners() {
    window.addEventListener('online', updateOnlineStatus);
    window.addEventListener('offline', updateOnlineStatus);
    updateOnlineStatus();

    document.getElementById('save-btn').addEventListener('click', handleSaveNote);
    document.getElementById('notes-list').addEventListener('click', handleNoteActions);
}

function registerServiceWorker() {
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('sw.js')
            .then(() => console.log('Service Worker зарегистрирован'))
            .catch(err => console.error('Ошибка Service Worker:', err));
    }
}

function handleSaveNote() {
    const titleInput = document.getElementById('title-input');
    const contentInput = document.getElementById('content-input');

    const title = titleInput.value.trim();
    const content = contentInput.value.trim();

    if (!title && !content) return;

    const notes = JSON.parse(localStorage.getItem('notes') || '[]');

    if (isEditing) {
        const index = notes.findIndex(n => n.id === currentEditId);
        if (index !== -1) {
            notes[index] = {
                ...notes[index],
                title: title || 'Без названия',
                content: content || ''
            };
            resetForm();
        }
    } else {
        notes.push({
            id: Date.now(),
            title: title || 'Без названия',
            content: content || '',
            created: new Date().toISOString()
        });
    }

    localStorage.setItem('notes', JSON.stringify(notes));
    resetForm();
    renderNotes(notes);
}

function handleNoteActions(e) {
    if (e.target.classList.contains('delete-btn')) {
        const id = Number(e.target.dataset.id);
        deleteNote(id);
    } else if (e.target.closest('.note-content')) {
        const id = Number(e.target.closest('.note-content').dataset.id);
        startEdit(id);
    }
}

function renderNotes(notes) {
    const list = document.getElementById('notes-list');
    list.innerHTML = '';

    notes.sort((a, b) => new Date(b.created) - new Date(a.created)).forEach(note => {
        const noteElement = document.createElement('div');
        noteElement.className = 'note';
        noteElement.innerHTML = `
            <div class="note-content" data-id="${note.id}">
                <div class="note-header">${note.title}</div>
                <div class="note-body">${note.content}</div>
            </div>
            <button class="delete-btn" data-id="${note.id}">Удалить</button>
        `;
        list.appendChild(noteElement);
    });
}

function deleteNote(id) {
    const notes = JSON.parse(localStorage.getItem('notes') || '[]')
        .filter(note => note.id !== id);
    localStorage.setItem('notes', JSON.stringify(notes));
    renderNotes(notes);
}

function startEdit(id) {
    const notes = JSON.parse(localStorage.getItem('notes') || '[]');
    const note = notes.find(n => n.id === id);
    if (note) {
        document.getElementById('title-input').value = note.title;
        document.getElementById('content-input').value = note.content;
        document.getElementById('save-btn').textContent = 'Сохранить';
        isEditing = true;
        currentEditId = id;
    }
}

function resetForm() {
    document.getElementById('title-input').value = '';
    document.getElementById('content-input').value = '';
    document.getElementById('save-btn').textContent = 'Добавить';
    isEditing = false;
    currentEditId = null;
}

function updateOnlineStatus() {
    const alert = document.getElementById('offline-alert');
    const container = document.querySelector('.container');

    if (!navigator.onLine) {
        alert.classList.add('active');
        container.style.paddingTop = alert.offsetHeight + 'px';
    } else {
        alert.classList.remove('active');
        container.style.paddingTop = '16px';
    }
}