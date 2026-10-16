/* 完整静态身体包含双臂与双脚，材质由原图决定，不染色或局部变形。 */
(() => {
  window.AllyBody = {
    resolve(style = 'original') {
      return window.AllyBodies[style] || window.AllyBodies.original;
    }
  };
})();
