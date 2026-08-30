allprojects { group = "dev.ghanageo"; version = "2.0.0-SNAPSHOT"; repositories { mavenCentral() } }
subprojects {
  apply(plugin = "java")
  extensions.configure<JavaPluginExtension> { withSourcesJar(); withJavadocJar() }
  tasks.withType<JavaCompile>().configureEach { options.release.set(21); options.encoding = "UTF-8"; options.compilerArgs.addAll(listOf("-Xlint:all,-processing", "-Werror")) }
  tasks.withType<Test>().configureEach { useJUnitPlatform() }
  tasks.withType<Javadoc>().configureEach { (options as StandardJavadocDocletOptions).addStringOption("Xdoclint:all,-missing", "-quiet"); isFailOnError = true }
}
project(":client") { apply(plugin = "java-library"); dependencies { "api"("com.fasterxml.jackson.core:jackson-databind:2.18.3"); "testImplementation"(platform("org.junit:junit-bom:5.12.1")); "testImplementation"("org.junit.jupiter:junit-jupiter"); "testRuntimeOnly"("org.junit.platform:junit-platform-launcher:1.12.1") } }
project(":spring-boot-starter") { dependencies { "implementation"(project(":client")); "implementation"("org.springframework.boot:spring-boot-autoconfigure:3.4.4"); "annotationProcessor"("org.springframework.boot:spring-boot-configuration-processor:3.4.4"); "testImplementation"(platform("org.junit:junit-bom:5.12.1")); "testImplementation"("org.junit.jupiter:junit-jupiter"); "testRuntimeOnly"("org.junit.platform:junit-platform-launcher:1.12.1"); "testImplementation"("org.springframework.boot:spring-boot-test:3.4.4"); "testImplementation"("org.assertj:assertj-core:3.27.3") } }
project(":examples") { dependencies { "implementation"(project(":client")) } }
project(":conformance") { dependencies { "implementation"(project(":client")) } }
