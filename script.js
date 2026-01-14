// Initialize AOS
AOS.init({
    duration: 1000,
    once: true,
    offset: 100
});

// Typed.js Initialization
const typed = new Typed('#typed-text', {
    strings: ['Freelance Web Developer', 'UI/UX Enthusiast', 'Problem Solver'],
    typeSpeed: 50,
    backSpeed: 30,
    backDelay: 2000,
    loop: true
});

// Mobile Menu
const hamburger = document.querySelector('.hamburger');
const navLinks = document.querySelector('.nav-links');

hamburger.addEventListener('click', () => {
    navLinks.classList.toggle('active');
    hamburger.classList.toggle('active'); // Add animation class if needed
});

// Close menu when link is clicked
document.querySelectorAll('.nav-links a').forEach(link => {
    link.addEventListener('click', () => {
        navLinks.classList.remove('active');
    });
});

// Skill Bar Animation using Intersection Observer
const skillsSection = document.querySelector('#skills');
const progressBars = document.querySelectorAll('.progress-fill');

function showProgress() {
    progressBars.forEach(bar => {
        const value = bar.getAttribute('data-width');
        bar.style.width = value + '%';
    });
}

function hideProgress() {
    progressBars.forEach(bar => {
        bar.style.width = '0';
    });
}

const skillsObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            showProgress();
        }
    });
}, { threshold: 0.2 });

if (skillsSection) {
    skillsObserver.observe(skillsSection);
}
