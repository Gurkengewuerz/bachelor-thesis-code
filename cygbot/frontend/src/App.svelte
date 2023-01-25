<script lang="ts">
    import {onMount} from 'svelte';

    import {EventsEmit, EventsOnMultiple, BrowserOpenURL} from '../wailsjs/runtime/runtime'
    import {GetPorts} from '../wailsjs/go/backend/App'

    import * as THREE from 'three';

    import {OrbitControls} from 'three/addons/controls/OrbitControls.js';
    import {PCDLoader} from 'three/addons/loaders/PCDLoader.js';

    let lastRunning = 0;
    $: isRunning = lastRunning > 0 && (new Date().getTime()) - lastRunning <= 2500;

    function connect(): void {
        if (!sensorPort || !servoPort) return;
        EventsEmit("start", {sensorPort, servoPort, fixRotation: fixRotation === "" ? -1 : parseInt(fixRotation)} as any);
    }

    function disconnect(): void {
        EventsEmit("stop", undefined);
    }

    let ports = [];
    let direction = 0;
    let servoPort = "";
    let sensorPort = "";
    let fixRotation = "";

    let camera, scene, renderer, htmlScene;

    function init() {
        renderer = new THREE.WebGLRenderer({antialias: true});
        renderer.setPixelRatio(window.devicePixelRatio);
        renderer.setSize(htmlScene.offsetWidth, htmlScene.offsetHeight);
        console.log(htmlScene.offsetHeight);
        htmlScene.appendChild(renderer.domElement);

        scene = new THREE.Scene();

        camera = new THREE.PerspectiveCamera(30, htmlScene.offsetWidth / htmlScene.offsetHeight, 0.01, 40);
        camera.position.set(0, 0, 1);
        scene.add(camera);

        const controls = new OrbitControls(camera, renderer.domElement);
        controls.addEventListener('change', render); // use if there is no animation loop
        //controls.minDistance = 0.5;
        //controls.maxDistance = 10;
/*
        const loader = new PCDLoader(undefined);
        loader.load('https://threejs.org/examples/models/pcd/binary/Zaghetto.pcd', function (points) {
            points.geometry.center();
            points.geometry.rotateX(Math.PI);
            points.name = 'Zaghetto.pcd';
            scene.add(points);

            render();
        }, function (xhr) {
            console.log((xhr.loaded / xhr.total * 100) + '% loaded');
        }, function (error) {
            console.log('An error happened');
        });
 */
    }

    function onWindowResize() {
        camera.aspect = htmlScene.offsetWidth / htmlScene.offsetHeight;
        camera.updateProjectionMatrix();
        renderer.setSize(htmlScene.offsetWidth, htmlScene.offsetHeight);
        render();
    }

    function render() {
        renderer.render(scene, camera);
    }

    EventsOnMultiple("stopped", () => {
        lastRunning = 0;
    }, undefined);

    EventsOnMultiple("direction", data => {
        direction = parseInt(data);
        lastRunning = new Date().getTime();
    }, undefined);

    EventsOnMultiple("data", data => {
        const enc = new TextEncoder();
        const loader = new PCDLoader(undefined);
        const obj = loader.parse(enc.encode(data));
        if (!obj) return;
        scene.clear();
        obj.geometry.center();
        scene.add(obj);
        render();
    }, undefined);

    onMount(async () => {
        GetPorts().then(data => {
            ports = JSON.parse(data);
        });

        init();
        render();

        window.addEventListener('resize', onWindowResize);
        return () => {
            window.removeEventListener('resize', onWindowResize);
        }
    })
</script>

<div class="container-fluid p-0 overflow-hidden">
    <div class="row">
        <div class="col col-9 p-0">
            <div class="vh-100 w-100 p-0" bind:this={htmlScene}></div>
        </div>
        <div class="col p-3">
            <form>

                <div class="form-group">
                    <label for="servo-com">Servo COM-Port</label>
                    <select class="form-control form-control-sm" id="servo-com" bind:value={servoPort}
                            disabled={isRunning}>
                        {#each ports as port}
                            <option>{port}</option>
                        {/each}
                    </select>
                </div>

                <div class="form-group mt-4">
                    <label for="sensor-com">Sensor COM-Port</label>
                    <select class="form-control form-control-sm" id="sensor-com" bind:value={sensorPort}
                            disabled={isRunning}>
                        {#each ports as port}
                            <option>{port}</option>
                        {/each}
                    </select>
                </div>

                <div class="form-group mt-4">
                    <label for="fix-rotation">Fix Rotation</label>
                    <input type="number" class="form-control" id="fix-rotation" bind:value={fixRotation}
                           placeholder="Rotate" min="0" max="180" disabled={isRunning}/>
                </div>

                <div class="form-group mt-4">
                    <span>Rotation Position ({direction}°)</span>
                    <div class="clock">
                        <div class="outer-clock-face">
                            <div class="marking marking-one"></div>
                            <div class="marking marking-two"></div>
                            <div class="marking marking-three"></div>
                            <div class="marking marking-four"></div>
                            <div class="inner-clock-face">
                                <div class="hand second-hand" style={`transform: rotate(${direction + 90}deg)`}></div>
                            </div>
                        </div>
                    </div>
                </div>
            </form>
            <button class={`mt-4 btn btn-sm btn w-100 ${isRunning ? "btn-danger" : "btn-success"}`}
                    on:click={isRunning ? disconnect : connect}>{isRunning ? "Disconnect" : "Connect"}</button>

            <div class="position-absolute bottom-0 text-center" style="font-size: 0.8rem; cursor: pointer" on:click={e => {
                e.preventDefault();
                BrowserOpenURL("https://mc8051.de?ref=ba-cygbot");
            }}>Coded with 💗 by Niklas Schütrumpf</div>
        </div>
    </div>
</div>
