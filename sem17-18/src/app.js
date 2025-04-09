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
    const input = document.getElementById('note-input');
    const content = input.value.trim();
    if (!content) return;

    const notes = JSON.parse(localStorage.getItem('notes') || '[]');

    if (isEditing) {
        const index = notes.findIndex(n => n.id === currentEditId);
        if (index !== -1) {
            notes[index].content = content;
            isEditing = false;
            currentEditId = null;
            document.getElementById('save-btn').textContent = 'Добавить';
        }
    } else {
        notes.push({
            id: Date.now(),
            content,
            created: new Date().toISOString()
        });
    }

    localStorage.setItem('notes', JSON.stringify(notes));
    input.value = '';
    renderNotes(notes);
}

function handleNoteActions(e) {
    if (e.target.classList.contains('delete-btn')) {
        const id = Number(e.target.dataset.id);
        deleteNote(id);
    } else if (e.target.classList.contains('note-content')) {
        const id = Number(e.target.dataset.id);
        startEdit(id);
    }
}

function renderNotes(notes) {
    const list = document.getElementById('notes-list');
    list.innerHTML = '';

    notes.forEach(note => {
        const noteElement = document.createElement('div');
        noteElement.className = 'note';
        noteElement.innerHTML = `
            <div class="note-content" data-id="${note.id}">${note.content}</div>
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
        document.getElementById('note-input').value = note.content;
        document.getElementById('save-btn').textContent = 'Сохранить';
        isEditing = true;
        currentEditId = id;
    }
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