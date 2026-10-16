/* 形象状态特效：与角色共享时钟，前后分层的轨道、侧边声纹与短暂散落星光。 */
(() => {
  const NS = 'http://www.w3.org/2000/svg';
  const clamp = value => Math.max(0, Math.min(1, value));
  function node(tag, attrs, parent) {
    const element = document.createElementNS(NS, tag);
    for (const [key, value] of Object.entries(attrs)) element.setAttribute(key, value);
    parent.appendChild(element);
    return element;
  }
  function create({ back, front }) {
    const behind = node('g', { class: 'ally-effects-back', 'pointer-events': 'none' }, back);
    const ahead = node('g', { class: 'ally-effects-front', 'pointer-events': 'none' }, front);
    const groups = {};
    const group = state => groups[state] = {
      rear: node('g', { 'data-effect': state, opacity: 0 }, behind),
      fore: node('g', { 'data-effect': state, opacity: 0 }, ahead),
      opacity: 0,
    };
    const ink = 'var(--companion-effect-strong)';
    const soft = 'var(--companion-effect-soft)';
    const stroke = (d, color, parent, width = 6) => node('path', { d, fill: 'none', stroke: color, 'stroke-width': width, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' }, parent);
    const thoughts = group('30');
    const dots = [-1, 0, 1].map(i => node('circle', { cx: 120 + i * 34, cy: -18, r: 9, fill: ink }, thoughts.fore));
    const orbitGroups = ['32', '36'].map(state => {
      const layer = group(state);
      return { state, tracks: [0, 1].map(i => ({ rear: stroke('', i ? soft : ink, layer.rear), fore: stroke('', i ? soft : ink, layer.fore) })) };
    });
    const search = group('40');
    const lenses = [-1, 1].map(side => {
      const lens = node('g', {}, search.fore);
      const ripple = node('circle', { r: 26, fill: 'none', stroke: soft, 'stroke-width': 3 }, lens);
      node('circle', { r: 17, fill: 'none', stroke: ink, 'stroke-width': 6 }, lens);
      stroke('M12 12 L28 28', ink, lens, 7);
      return { lens, ripple, side };
    });
    const voiceGroups = ['35', '39'].map(state => {
      const layer = group(state);
      return { state, bars: [-1, 1].flatMap(side => [0, 1, 2, 3, 4].map(i => ({ element: stroke('', i % 2 ? soft : ink, layer.fore), side, i }))) };
    });
    const error = group('34');
    stroke('M-5 65 L-23 85 L-4 92 L-20 115', 'var(--companion-effect-deep)', error.fore, 7);
    stroke('M245 65 L263 85 L244 92 L260 115', 'var(--companion-effect-deep)', error.fore, 7);
    const starPath = 'M0 -10 Q2 -2 10 0 Q2 2 0 10 Q-2 2 -10 0 Q-2 -2 0 -10Z';
    const stars = ['31', '33'].map(state => {
      const layer = group(state);
      return { state, points: Array.from({ length: state === '33' ? 12 : 6 }, (_, i) => node('path', { d: starPath, fill: i % 2 ? soft : ink }, layer.fore)) };
    });
    const groupEntries = Object.entries(groups);
    let previousTime = 0;
    function update({ emotion, elapsed, time, reduced, x, y, rotate }) {
      const dt = Math.min(.06, Math.max(0, time - previousTime));
      previousTime = time;
      const t = reduced ? .6 : elapsed;
      let hasVisibleEffect = false;
      for (const [state, layer] of groupEntries) {
        // 特效入场与退出都从当前透明度衔接，新消息打断时不会突然闪烁。
        const target = emotion === state ? 1 : 0;
        const previous = layer.opacity;
        let next = reduced ? target : previous + (target - previous) * (1 - Math.exp(-dt * 14));
        if (Math.abs(target - next) < .001) next = target;
        layer.opacity = next;
        if (next !== previous) {
          layer.rear.setAttribute('opacity', next);
          layer.fore.setAttribute('opacity', next);
        }
        if (next > .001) hasVisibleEffect = true;
      }
      if (!hasVisibleEffect) return;
      // 特效坐标可以超出 240 单位画布；只扩大轨道和位移，不缩小身体来腾空间。
      const transform = `translate(${x} ${y}) rotate(${rotate * .35} 120 135)`;
      behind.setAttribute('transform', transform);
      ahead.setAttribute('transform', transform);
      if (thoughts.opacity > .001) {
        dots.forEach((dot, i) => {
          const pulse = reduced ? .6 : (Math.sin(t * 4 - i * 1.2) + 1) / 2;
          dot.setAttribute('r', 8 + pulse * 5);
          dot.setAttribute('cy', -12 - pulse * 15);
          dot.setAttribute('opacity', .6 + pulse * .4);
        });
      }
      for (const orbit of orbitGroups) orbit.tracks.forEach((track, index) => {
        if (groups[orbit.state].opacity <= .001) return;
        // 椭圆轨道按深度拆开，后半圈被身体遮住，前半圈从腰部穿过。
        let rear = '', fore = '', previousBehind;
        const phase = t * (orbit.state === '36' ? 1.6 : 2.7) + index * Math.PI;
        for (let step = 0; step <= 44; step++) {
          const angle = phase + step / 44 * Math.PI * 1.6;
          const px = Math.cos(angle) * 190;
          const py = Math.sin(angle) * 52;
          const tilt = index ? -.28 : .28;
          const cx = 120 + px * Math.cos(tilt) - py * Math.sin(tilt);
          const cy = 150 + px * Math.sin(tilt) + py * Math.cos(tilt);
          const isBehind = Math.sin(angle) < 0;
          const segment = `${previousBehind === isBehind ? 'L' : 'M'}${cx.toFixed(2)} ${cy.toFixed(2)}`;
          if (isBehind) rear += segment; else fore += segment;
          previousBehind = isBehind;
        }
        track.rear.setAttribute('d', rear);
        track.fore.setAttribute('d', fore);
      });
      if (search.opacity > .001) {
        lenses.forEach(({ lens, ripple, side }) => {
          const pulse = reduced ? .5 : (Math.sin(t * 2 + side) + 1) / 2;
          lens.setAttribute('transform', `translate(${120 + side * (148 + pulse * 16)} ${85 + Math.sin(t * 2 + side) * (reduced ? 0 : 42)}) rotate(${side * (pulse - .5) * 22})`);
          lens.setAttribute('opacity', .7 + pulse * .3);
          ripple.setAttribute('r', 24 + pulse * 14);
          ripple.setAttribute('opacity', .55 - pulse * .3);
        });
      }
      voiceGroups.forEach(({ state, bars }) => {
        if (groups[state].opacity <= .001) return;
        bars.forEach(({ element, side, i }) => {
          const envelope = 1 - Math.abs(i - 2) * .18;
          const height = (reduced ? 25 : 12 + (Math.sin(t * (state === '35' ? 2.8 : 6) - i * .8) + 1) * 17) * envelope;
          const px = 120 + side * (124 + i * 13);
          element.setAttribute('d', `M${px} ${116 - height} V${116 + height}`);
        });
      });
      stars.forEach(({ state, points }) => {
        if (groups[state].opacity <= .001) return;
        points.forEach((star, i) => {
          const progress = reduced ? .45 : clamp(t / (state === '33' ? 1.8 : .7));
          const angle = i / points.length * Math.PI * 2 - Math.PI / 2;
          const radius = 90 + progress * 130;
          star.setAttribute('transform', `translate(${120 + Math.cos(angle) * radius} ${100 + Math.sin(angle) * radius * .85 + progress * progress * 18}) rotate(${progress * 70}) scale(${1 + Math.sin(progress * Math.PI) * .8})`);
          star.setAttribute('opacity', reduced ? .85 : Math.sin(progress * Math.PI));
        });
      });
    }
    return { update };
  }
  window.AllyCompanionEffects = { create };
})();
