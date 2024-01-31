/** @type {import('tailwindcss').Config} */

module.exports = {
    content: ["./internal/pkg/presentation/web/components/**/*.templ"],
    darkMode: 'class',
    theme: {
      extend: {
        fontFamily: {
          heading: ['Raleway'],
          sans: ['Arial', 'sans-serif'],
        }  
      },
    },
    plugins: [],
  }
  